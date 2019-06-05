package cache

import (
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	json "github.com/mailru/easyjson"
	"github.com/prometheus/common/log"
)

const (
	APP_BUCKET = "AppBucket"
)

var (
	MissingAndIgnoredErr = errors.New("App was missed and ignored")
)

type BoltdbConfig struct {
	Path               string
	IgnoreMissingApps  bool
	MissingAppCacheTTL time.Duration
	AppCacheTTL        time.Duration
	OrgSpaceCacheTTL   time.Duration
	AppLimits          int
}

// Org is a CAPI org
type Org struct {
	Name        string
	LastUpdated time.Time
}

// Space is a CAPI space with a reference to its org's GUID
type Space struct {
	Name        string
	OrgGUID     string
	LastUpdated time.Time
}

type Boltdb struct {
	appClient AppClient
	appdb     *bolt.DB

	lock        sync.RWMutex
	cache       map[string]*App
	missingApps map[string]struct{}

	orgNameCache   map[string]Org   // caches org guid->org name mapping
	spaceNameCache map[string]Space // caches space guid->space name mapping

	closing chan struct{}
	wg      sync.WaitGroup
	config  *BoltdbConfig
}

func NewBoltdb(client AppClient, config *BoltdbConfig) (*Boltdb, error) {
	return &Boltdb{
		appClient:      client,
		cache:          make(map[string]*App),
		missingApps:    make(map[string]struct{}),
		orgNameCache:   make(map[string]Org),
		spaceNameCache: make(map[string]Space),
		closing:        make(chan struct{}),
		config:         config,
	}, nil
}

func (c *Boltdb) Open() error {
	// Open bolt db
	db, err := bolt.Open(c.config.Path, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		log.Error("Fail to open boltdb: ", err)
		return err
	}
	c.appdb = db

	if err := c.createBucket(); err != nil {
		log.Error("Fail to create bucket: ", err)
		return err
	}

	if c.config.AppCacheTTL != time.Duration(0) {
		c.invalidateCache()
	}

	if c.config.MissingAppCacheTTL != time.Duration(0) {
		c.invalidateMissingAppCache()
	}

	return c.populateCache()
}

func (c *Boltdb) populateCache() error {
	apps, err := c.getAllAppsFromBoltDB()
	if err != nil {
		return err
	}

	if len(apps) == 0 {
		// populate from remote
		apps, err = c.getAllAppsFromRemote()
		if err != nil {
			return err
		}
	}

	c.cache = apps

	return nil
}

func (c *Boltdb) Close() error {
	close(c.closing)

	// Wait for background goroutine exit
	c.wg.Wait()

	return c.appdb.Close()
}

// GetApp tries first get app info from cache. If caches doesn't have this
// app info (cache miss), it issues API to retrieve the app info from remote
// if the app is not already missing and clients don't ignore the missing app
// info, and then add the app info to the cache
// On the other hand, if the app is already missing and clients want to
// save remote API and ignore missing app, then a nil app info and an error
// will be returned.
func (c *Boltdb) GetApp(appGuid string) (*App, error) {
	app, err := c.getAppFromCache(appGuid)
	if err != nil {
		return nil, err
	}

	// Find in cache
	if app != nil {
		c.fillOrgAndSpace(app)
		return app, nil
	}

	// First time seeing app
	app, err = c.getAppFromRemote(appGuid)
	if err != nil {
		if c.config.IgnoreMissingApps {
			// Record this missing app
			c.lock.Lock()
			c.missingApps[appGuid] = struct{}{}
			c.lock.Unlock()
		}
		return nil, err
	}

	// Add to in-memory cache
	c.lock.Lock()
	c.cache[app.Guid] = app
	c.lock.Unlock()

	return app, nil
}

// GetAllApps returns all apps info
func (c *Boltdb) GetAllApps() (map[string]*App, error) {
	c.lock.RLock()
	apps := make(map[string]*App, len(c.cache))
	for _, app := range c.cache {
		dup := *app
		apps[dup.Guid] = &dup
	}
	c.lock.RUnlock()

	return apps, nil
}

func (c *Boltdb) ManuallyInvalidateCaches() error {
	c.lock.Lock()
	c.orgNameCache = make(map[string]Org)
	c.spaceNameCache = make(map[string]Space)
	c.lock.Unlock()

	apps, err := c.getAllAppsFromRemote()
	if err != nil {
		return err
	}
	c.lock.Lock()
	c.cache = apps
	c.lock.Unlock()
	return nil
}

func (c *Boltdb) getAppFromCache(appGuid string) (*App, error) {
	c.lock.RLock()
	if app, ok := c.cache[appGuid]; ok {
		// in in-memory cache
		c.lock.RUnlock()
		return app, nil
	}

	_, alreadyMissed := c.missingApps[appGuid]
	if c.config.IgnoreMissingApps && alreadyMissed {
		// already missed
		c.lock.RUnlock()
		return nil, MissingAndIgnoredErr
	}
	c.lock.RUnlock()

	// Didn't find in cache and it is not missed or we are not ignoring missed app
	return nil, nil
}

func (c *Boltdb) getAllAppsFromBoltDB() (map[string]*App, error) {
	var allData [][]byte
	c.appdb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(APP_BUCKET))
		b.ForEach(func(guid []byte, v []byte) error {
			allData = append(allData, v)
			return nil
		})
		return nil
	})

	apps := make(map[string]*App, len(allData))
	for i := range allData {
		var app App
		err := json.Unmarshal(allData[i], &app)
		if err != nil {
			return nil, err
		}
		apps[app.Guid] = &app
	}

	return apps, nil
}

func (c *Boltdb) getAllAppsFromRemote() (map[string]*App, error) {
	log.Info("Retrieving apps from remote")

	totalPages := 0
	q := url.Values{}
	q.Set("inline-relations-depth", "0")
	if c.config.AppLimits > 0 {
		// Latest N apps
		q.Set("order-direction", "desc")
		q.Set("results-per-page", "100")
		totalPages = c.config.AppLimits/100 + 1
	}

	cfApps, err := c.appClient.ListAppsByQueryWithLimits(q, totalPages)
	if err != nil {
		return nil, err
	}

	apps := make(map[string]*App, len(cfApps))
	for i := range cfApps {
		app := c.fromPCFApp(&cfApps[i])
		apps[app.Guid] = app
	}

	c.fillDatabase(apps)

	log.Info(fmt.Sprintf("Found %d apps", len(apps)))

	return apps, nil
}

func (c *Boltdb) createBucket() error {
	return c.appdb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(APP_BUCKET))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
}

// invalidateMissingAppCache perodically cleanup inmemory house keeping for
// not found apps. When the this cache is cleaned up, end clients have chance
// to retry missing apps
func (c *Boltdb) invalidateMissingAppCache() {
	ticker := time.NewTicker(c.config.MissingAppCacheTTL)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		for {
			select {
			case <-ticker.C:
				c.lock.Lock()
				c.missingApps = make(map[string]struct{})
				c.lock.Unlock()
			case <-c.closing:
				return
			}
		}
	}()
}

// invalidateCache perodically fetches a full copy apps info from remote
// and update boltdb and in-memory cache
func (c *Boltdb) invalidateCache() {
	ticker := time.NewTicker(c.config.AppCacheTTL)
	orgSpaceTicker := time.NewTicker(c.config.OrgSpaceCacheTTL)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		for {
			select {
			case <-ticker.C:
				apps, err := c.getAllAppsFromRemote()
				if err == nil {
					c.lock.Lock()
					c.cache = apps
					c.lock.Unlock()
				} else {
					log.Error("Unable to fetch copy of cache from remote", err)
				}
			case <-orgSpaceTicker.C:
				c.lock.Lock()
				c.orgNameCache = make(map[string]Org)
				c.spaceNameCache = make(map[string]Space)
				c.lock.Unlock()
			case <-c.closing:
				return
			}
		}
	}()
}

func (c *Boltdb) fillDatabase(apps map[string]*App) {
	for _, app := range apps {
		c.appdb.Update(func(tx *bolt.Tx) error {
			serialize, err := json.Marshal(app)
			if err != nil {
				return fmt.Errorf("Error Marshaling data: %s", err)
			}

			b := tx.Bucket([]byte(APP_BUCKET))
			if err := b.Put([]byte(app.Guid), serialize); err != nil {
				return fmt.Errorf("Error inserting data: %s", err)
			}
			return nil
		})
	}
}

func (c *Boltdb) fromPCFApp(app *cfclient.App) *App {
	cachedApp := &App{
		Name:       app.Name,
		Guid:       app.Guid,
		SpaceGuid:  app.SpaceGuid,
		IgnoredApp: c.isOptOut(app.Environment),
	}

	c.fillOrgAndSpace(cachedApp)

	return cachedApp
}

func (c *Boltdb) fillOrgAndSpace(app *App) error {
	now := time.Now()

	c.lock.RLock()
	space, ok := c.spaceNameCache[app.SpaceGuid]
	c.lock.RUnlock()

	if !ok || now.Sub(space.LastUpdated) > c.config.OrgSpaceCacheTTL {
		cfspace, err := c.appClient.GetSpaceByGuid(app.SpaceGuid)
		if err != nil {
			return err
		}

		space = Space{
			Name:        cfspace.Name,
			OrgGUID:     cfspace.OrganizationGuid,
			LastUpdated: now,
		}

		c.lock.Lock()
		c.spaceNameCache[app.SpaceGuid] = space
		c.lock.Unlock()
	}

	app.SpaceName = space.Name
	app.OrgGuid = space.OrgGUID

	c.lock.RLock()
	org, ok := c.orgNameCache[space.OrgGUID]
	c.lock.RUnlock()
	if !ok || now.Sub(org.LastUpdated) > c.config.OrgSpaceCacheTTL {
		cforg, err := c.appClient.GetOrgByGuid(space.OrgGUID)
		if err != nil {
			return err
		}

		org = Org{
			Name:        cforg.Name,
			LastUpdated: now,
		}

		c.lock.Lock()
		c.orgNameCache[space.OrgGUID] = org
		c.lock.Unlock()
	}

	app.OrgGuid = space.OrgGUID
	app.OrgName = org.Name

	return nil
}

func (c *Boltdb) getAppFromRemote(appGuid string) (*App, error) {
	cfApp, err := c.appClient.AppByGuid(appGuid)
	if err != nil {
		return nil, err
	}
	app := c.fromPCFApp(&cfApp)
	c.fillDatabase(map[string]*App{app.Guid: app})

	return app, nil
}

func (c *Boltdb) isOptOut(envVar map[string]interface{}) bool {
	if val, ok := envVar["F2S_DISABLE_LOGGING"]; ok && val == "true" {
		return true
	}
	return false
}
