package messages

import (
	"fmt"

	"github.com/bosh-loki/loki-firehose-nozzle/utils"
	"github.com/cloudfoundry/sonde-go/events"
)

func GetMessage(e *events.Envelope) (LabelSet, string) {
	switch e.GetEventType() {
	case events.Envelope_HttpStartStop:
		return newHTTPStartStop(e)
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
	return nil, ""
}

// newLogMessage creates a new newLogMessage
func newLogMessage(e *events.Envelope) (LabelSet, string) {
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
	return r, msg
}

// newValueMetric creates a new newValueMetric
func newValueMetric(e *events.Envelope) (LabelSet, string) {
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
	return r, msg
}

// newHttpStartStop creates a new newHttpStartStop
func newHTTPStartStop(e *events.Envelope) (LabelSet, string) {
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
	return r, msg
}

// newContainerMetric creates a new newContainerMetric
func newContainerMetric(e *events.Envelope) (LabelSet, string) {
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
	return r, msg
}

// newCounterEvent creates a new newCounterEvent
func newCounterEvent(e *events.Envelope) (LabelSet, string) {
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
	return r, msg
}

// newErrorEvent creates a new newErrorEvent
func newErrorEvent(e *events.Envelope) (LabelSet, string) {
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
	return r, msg
}
