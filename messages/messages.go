package messages

import (
	"fmt"

	"github.com/bosh-loki/loki-firehose-nozzle/cache"
	"github.com/bosh-loki/loki-firehose-nozzle/utils"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/prometheus/common/log"
)

type Event struct {
	Labels LabelSet
	Msg    string
}

func GetMessage(e *events.Envelope, c cache.Cache) *Event {
	var event *Event
	switch e.GetEventType() {
	case events.Envelope_HttpStartStop:
		event = newHTTPStartStop(e)
	case events.Envelope_LogMessage:
		event = newLogMessage(e)
	case events.Envelope_ContainerMetric:
		event = newContainerMetric(e)
	case events.Envelope_Error:
		event = newErrorEvent(e)
	case events.Envelope_ValueMetric:
		event = newValueMetric(e)
	case events.Envelope_CounterEvent:
		event = newCounterEvent(e)
	}
	if _, hasAppID := event.Labels["cf_app_id"]; hasAppID {
		AnnotateWithAppData(c, event)
	}
	return event
}

// newLogMessage creates a new newLogMessage
func newLogMessage(e *events.Envelope) *Event {
	var m = e.GetLogMessage()
	var r = LabelSet{
		"cf_app_id":       m.GetAppId(),
		"cf_origin":       "firehose",
		"deployment":      e.GetDeployment(),
		"event_type":      e.GetEventType().String(),
		"job":             e.GetJob(),
		"job_index":       e.GetIndex(),
		"message_type":    m.GetMessageType().String(),
		"origin":          e.GetOrigin(),
		"source_instance": m.GetSourceInstance(),
		"source_type":     m.GetSourceType(),
	}
	msg := string(m.GetMessage())
	return &Event{
		Labels: r,
		Msg:    msg,
	}
}

// newValueMetric creates a new newValueMetric
func newValueMetric(e *events.Envelope) *Event {
	var m = e.GetValueMetric()
	var r = LabelSet{
		"cf_origin":  "firehose",
		"deployment": e.GetDeployment(),
		"event_type": e.GetEventType().String(),
		"job":        e.GetJob(),
		"job_index":  e.GetIndex(),
		"origin":     e.GetOrigin(),
	}
	msg := fmt.Sprintf("%s = %g (%s)", m.GetName(), m.GetValue(), m.GetUnit())
	return &Event{
		Labels: r,
		Msg:    msg,
	}
}

// newHttpStartStop creates a new newHttpStartStop
func newHTTPStartStop(e *events.Envelope) *Event {
	var m = e.GetHttpStartStop()
	var r = LabelSet{
		"cf_origin":  "firehose",
		"deployment": e.GetDeployment(),
		"event_type": e.GetEventType().String(),
		"job":        e.GetJob(),
		"job_index":  e.GetIndex(),
		"origin":     e.GetOrigin(),
	}
	if m.ApplicationId != nil {
		r["cf_app_id"] = utils.FormatUUID(m.GetApplicationId())
	}
	if m.InstanceId != nil {
		r["instance_id"] = m.GetInstanceId()
	}
	msg := fmt.Sprintf("%d %s %s (%d ms)", m.GetStatusCode(), m.GetMethod(), m.GetUri(), ((m.GetStopTimestamp()-m.GetStartTimestamp())/1000)/1000)
	return &Event{
		Labels: r,
		Msg:    msg,
	}
}

// newContainerMetric creates a new newContainerMetric
func newContainerMetric(e *events.Envelope) *Event {
	var m = e.GetContainerMetric()
	var r = LabelSet{
		"cf_app_id":  m.GetApplicationId(),
		"cf_origin":  "firehose",
		"deployment": e.GetDeployment(),
		"event_type": e.GetEventType().String(),
		"job":        e.GetJob(),
		"job_index":  e.GetIndex(),
		"origin":     e.GetOrigin(),
	}
	msg := fmt.Sprintf("cpu_percentage=%g, memory_bytes=%d, disk_bytes=%d", m.GetCpuPercentage(), m.GetMemoryBytes(), m.GetDiskBytes())
	return &Event{
		Labels: r,
		Msg:    msg,
	}
}

// newCounterEvent creates a new newCounterEvent
func newCounterEvent(e *events.Envelope) *Event {
	var m = e.GetCounterEvent()
	var r = LabelSet{
		"cf_origin":  "firehose",
		"deployment": e.GetDeployment(),
		"event_type": e.GetEventType().String(),
		"job":        e.GetJob(),
		"job_index":  e.GetIndex(),
		"origin":     e.GetOrigin(),
	}
	msg := fmt.Sprintf("%s (delta=%d, total=%d)", m.GetName(), m.GetDelta(), m.GetTotal())
	return &Event{
		Labels: r,
		Msg:    msg,
	}
}

// newErrorEvent creates a new newErrorEvent
func newErrorEvent(e *events.Envelope) *Event {
	var m = e.GetError()
	var r = LabelSet{
		"cf_origin":  "firehose",
		"deployment": e.GetDeployment(),
		"event_type": e.GetEventType().String(),
		"job":        e.GetJob(),
		"job_index":  e.GetIndex(),
		"origin":     e.GetOrigin(),
	}
	msg := fmt.Sprintf("%d %s: %s", m.GetCode(), m.GetSource(), m.GetMessage())
	return &Event{
		Labels: r,
		Msg:    msg,
	}
}

func AnnotateWithAppData(appCache cache.Cache, e *Event) {
	cfAppID := e.Labels["cf_app_id"]
	appGUID := fmt.Sprintf("%s", cfAppID)

	if appGUID != "<nil>" && cfAppID != "" {
		appInfo, err := appCache.GetApp(appGUID)
		if err != nil {
			log.Errorf("Encountered an error while getting app info: %v", err)
		}

		cfAppName := appInfo.Name
		cfSpaceID := appInfo.SpaceGuid
		cfSpaceName := appInfo.SpaceName
		cfOrgID := appInfo.OrgGuid
		cfOrgName := appInfo.OrgName

		if cfAppName != "" {
			e.Labels["cf_app_name"] = cfAppName
		}

		if cfSpaceID != "" {
			e.Labels["cf_space_id"] = cfSpaceID
		}

		if cfSpaceName != "" {
			e.Labels["cf_space_name"] = cfSpaceName
		}

		if cfOrgID != "" {
			e.Labels["cf_org_id"] = cfOrgID
		}

		if cfOrgName != "" {
			e.Labels["cf_org_name"] = cfOrgName
		}
	}
}
