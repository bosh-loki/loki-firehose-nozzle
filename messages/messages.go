package messages

import (
	"strings"

	"github.com/bosh-loki/firehose-loki-client/utils"
	"github.com/cloudfoundry/sonde-go/events"
)

func GetLabels(e *events.Envelope) LabelSet {
	switch e.GetEventType() {
	case events.Envelope_HttpStartStop:
		return newHttpStartStop(e)
	case events.Envelope_LogMessage:
		return newLogMessage(e)
	case events.Envelope_ContainerMetric:
		return newContainerMetric(e)
	case events.Envelope_Error:
		return newErrorEvent(e)
	case events.Envelope_ValueMetric:
		return newValueMetric(e)
	case events.Envelope_CounterEvent:
		return newCounterEvent(e)
	}
	return nil
}

// newLogMessage creates a new newLogMessage
func newLogMessage(e *events.Envelope) LabelSet {
	var m = e.GetLogMessage()
	var r = LabelSet{
		"event_type":      e.GetEventType().String(),
		"cf_app_id":       m.GetAppId(),
		"source_type":     strings.Replace(m.GetSourceType(), "/", "-", -1),
		"message_type":    m.GetMessageType().String(),
		"source_instance": m.GetSourceInstance(),
	}
	return r
}

// newValueMetric creates a new newValueMetric
func newValueMetric(e *events.Envelope) LabelSet {
	var m = e.GetValueMetric()
	var r = LabelSet{
		"event_type": e.GetEventType().String(),
		"name":       m.GetName(),
	}
	return r
}

// newHttpStartStop creates a new newHttpStartStop
func newHttpStartStop(e *events.Envelope) LabelSet {
	var m = e.GetHttpStartStop()
	var r = LabelSet{
		"event_type":     e.GetEventType().String(),
		"cf_app_id":      utils.FormatUUID(m.GetApplicationId()),
		"instance_id":    m.GetInstanceId(),
		"instance_index": string(m.GetInstanceIndex()),
	}
	return r
}

// newContainerMetric creates a new newContainerMetric
func newContainerMetric(e *events.Envelope) LabelSet {
	var m = e.GetContainerMetric()
	var r = LabelSet{
		"event_type":     e.GetEventType().String(),
		"cf_app_id":      m.GetApplicationId(),
		"instance_index": string(m.GetInstanceIndex()),
	}
	return r
}

// newCounterEvent creates a new newCounterEvent
func newCounterEvent(e *events.Envelope) LabelSet {
	var m = e.GetCounterEvent()
	var r = LabelSet{
		"event_type": e.GetEventType().String(),
		"name":       m.GetName(),
	}
	return r
}

// newErrorEvent creates a new newErrorEvent
func newErrorEvent(e *events.Envelope) LabelSet {
	var m = e.GetError()
	var r = LabelSet{
		"event_type": e.GetEventType().String(),
		"source":     m.GetSource(),
	}
	return r
}
