package extralabels

import (
	"fmt"
	"strings"
)

func SetBaseLabels(baseLabelsString string) (map[string]string, error) {
	baseLabels := map[string]string{}

	for _, kvPair := range strings.Split(baseLabelsString, ",") {
		if kvPair != "" {
			cleaned := strings.TrimSpace(kvPair)
			k, v, err := getKeyValueFromString(cleaned)
			if err != nil {
				return nil, err
			}
			baseLabels[k] = v
		}
	}
	return baseLabels, nil
}

func getKeyValueFromString(kvPair string) (string, string, error) {
	values := strings.Split(kvPair, ":")
	if len(values) != 2 {
		return "", "", fmt.Errorf("When splitting %s by ':' there must be exactly 2 values, got these values %s", kvPair, values)
	}
	return strings.TrimSpace(values[0]), strings.TrimSpace(values[1]), nil
}
