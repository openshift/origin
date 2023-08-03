package monitorapi

func (o *Interval) DeepCopy() *Interval {
	ret := &Interval{
		Condition: *o.Condition.DeepCopy(),
		Source:    o.Source,
		Display:   o.Display,
		From:      o.From,
		To:        o.To,
	}

	return ret
}

func (o *Condition) DeepCopy() *Condition {
	ret := &Condition{
		Level:             o.Level,
		Locator:           o.Locator,
		StructuredLocator: *o.StructuredLocator.DeepCopy(),
		Message:           o.Message,
		StructuredMessage: *o.StructuredMessage.DeepCopy(),
	}

	return ret
}

func (o *Locator) DeepCopy() *Locator {
	ret := &Locator{
		Type: o.Type,
		Keys: make(map[LocatorKey]string, len(o.Keys)),
	}
	for k, v := range o.Keys {
		ret.Keys[k] = v
	}

	return ret
}

func (o *Message) DeepCopy() *Message {
	ret := &Message{
		Reason:       o.Reason,
		Cause:        o.Cause,
		HumanMessage: o.HumanMessage,
		Annotations:  make(map[AnnotationKey]string, len(o.Annotations)),
	}
	for k, v := range o.Annotations {
		ret.Annotations[k] = v
	}

	return ret
}
