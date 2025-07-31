package ipmi

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// GetSensorReadingReq represents a Get Sensor Reading command, specified in
// 29.14 and 35.14 of v1.5 and v2.0 respectively.
type GetSensorReadingReq struct {
	layers.BaseLayer

	// Number is the number of the sensor whose reading to retrieve. The sensor
	// number is specified in an SDR returned by the BMC. 0xff is reserved.
	Number uint8
}

func (*GetSensorReadingReq) LayerType() gopacket.LayerType {
	return LayerTypeGetSensorReadingReq
}

func (r *GetSensorReadingReq) SerializeTo(b gopacket.SerializeBuffer, _ gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(1)
	if err != nil {
		return err
	}
	bytes[0] = r.Number
	return nil
}

type GetSensorReadingRsp struct {
	layers.BaseLayer

	// Reading is the raw reading value, which can be unsigned, 1's complement
	// or 2's complement. A suitable AnalogDataFormatConverter implementation
	// can be obtained by calling .AnalogDataFormat.Converter() on
	// FullSensorRecord. If the sensor does not return a numeric reading or
	// ReadingUnavailable is set, this should be ignored.
	Reading byte

	// EventMessagesEnabled indicates whether all Event Messages are enabled for
	// the sensor.
	EventMessagesEnabled bool

	// ScanningEnabled indicates whether sensor scanning is enabled
	ScanningEnabled bool

	// ReadingUnavailable is used by some BMCs to indicate a sensor update is in
	// progress, or that the entity is not present. If set, the reading should
	// be ignored.
	ReadingUnavailable bool
}

func (*GetSensorReadingRsp) LayerType() gopacket.LayerType {
	return LayerTypeGetSensorReadingRsp
}

func (r *GetSensorReadingRsp) CanDecode() gopacket.LayerClass {
	return r.LayerType()
}

func (*GetSensorReadingRsp) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (r *GetSensorReadingRsp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 3 {
		// the sensor is likely inactive or disabled
		df.SetTruncated()
		return fmt.Errorf("response must be at least 2 bytes, got %v", len(data))
	}

	r.Reading = data[0]
	r.EventMessagesEnabled = data[1]&(1<<7) != 0
	r.ScanningEnabled = data[1]&(1<<6) != 0
	r.ReadingUnavailable = data[1]&(1<<5) != 0

	if len(data) > 3 {
		// discrete reading sensors only section
		r.BaseLayer.Contents = data[:4]
		r.BaseLayer.Payload = data[4:]
	} else {
		r.BaseLayer.Contents = data[:3]
		r.BaseLayer.Payload = data[3:]
	}
	return nil
}

type GetSensorReadingCmd struct {
	Req GetSensorReadingReq
	Rsp GetSensorReadingRsp

	// OwnerLUN is the remote LUN of the sensor being retrieved. We learn this
	// from the SDR.
	OwnerLUN LUN
}

// Name returns "Get Sensor Reading".
func (*GetSensorReadingCmd) Name() string {
	return "Get Sensor Reading"
}

// Operation returns &OperationGetSensorReadingReq.
func (*GetSensorReadingCmd) Operation() *Operation {
	return &OperationGetSensorReadingReq
}

func (c *GetSensorReadingCmd) RemoteLUN() LUN {
	return c.OwnerLUN
}

func (c *GetSensorReadingCmd) Request() gopacket.SerializableLayer {
	return &c.Req
}

func (c *GetSensorReadingCmd) Response() gopacket.DecodingLayer {
	return &c.Rsp
}
