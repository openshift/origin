package ipmi

import (
	"github.com/gebn/bmc/pkg/layerexts"

	"github.com/google/gopacket"
)

var (
	LayerTypeSessionSelector = gopacket.RegisterLayerType(
		1000,
		gopacket.LayerTypeMetadata{
			Name: "IPMI Session Selector",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &SessionSelector{}
			}),
		},
	)
	LayerTypeV1Session = gopacket.RegisterLayerType(
		1001,
		gopacket.LayerTypeMetadata{
			Name: "Session v1.5",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &V1Session{}
			}),
		},
	)
	LayerTypeGetChannelAuthenticationCapabilitiesReq = gopacket.RegisterLayerType(
		1002,
		gopacket.LayerTypeMetadata{
			Name: "Get Channel Authentication Capabilities Request",
		},
	)
	LayerTypeGetChannelAuthenticationCapabilitiesRsp = gopacket.RegisterLayerType(
		1003,
		gopacket.LayerTypeMetadata{
			Name: "Get Channel Authentication Capabilities Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &GetChannelAuthenticationCapabilitiesRsp{}
			}),
		},
	)
	LayerTypeV2Session = gopacket.RegisterLayerType(
		1004,
		gopacket.LayerTypeMetadata{
			Name: "Session v2.0",
			// by default this layer can only encode and decode unauthenticated
			// packets; to deal with authenticated packets, the
			// IntegrityAlgorithm attribute must be set
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &V2Session{}
			}),
		},
	)
	LayerTypeOpenSessionReq = gopacket.RegisterLayerType(
		1005,
		gopacket.LayerTypeMetadata{
			Name: "RMCP+ Open Session Request",
		},
	)
	LayerTypeOpenSessionRsp = gopacket.RegisterLayerType(
		1006,
		gopacket.LayerTypeMetadata{
			Name: "RMCP+ Open Session Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &OpenSessionRsp{}
			}),
		},
	)
	LayerTypeRAKPMessage1 = gopacket.RegisterLayerType(
		1007,
		gopacket.LayerTypeMetadata{
			Name: "RAKP Message 1",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &RAKPMessage1{}
			}),
		},
	)
	LayerTypeRAKPMessage2 = gopacket.RegisterLayerType(
		1008,
		gopacket.LayerTypeMetadata{
			Name: "RAKP Message 2",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &RAKPMessage2{}
			}),
		},
	)
	LayerTypeRAKPMessage3 = gopacket.RegisterLayerType(
		1009,
		gopacket.LayerTypeMetadata{
			Name: "RAKP Message 3",
		},
	)
	LayerTypeRAKPMessage4 = gopacket.RegisterLayerType(
		1010,
		gopacket.LayerTypeMetadata{
			Name: "RAKP Message 4",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &RAKPMessage4{}
			}),
		},
	)
	layerTypeAES128CBC = gopacket.RegisterLayerType(
		1011,
		gopacket.LayerTypeMetadata{
			Name: "AES-128-CBC Encrypted IPMI Message",
			// decoder not specified here as default struct not usable
		},
	)
	LayerTypeMessage = gopacket.RegisterLayerType(
		1012,
		gopacket.LayerTypeMetadata{
			Name: "IPMI Message",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &Message{}
			}),
		},
	)
	LayerTypeCloseSessionReq = gopacket.RegisterLayerType(
		1013,
		gopacket.LayerTypeMetadata{
			Name: "Close Session Request",
		},
	)
	LayerTypeGetSystemGUIDRsp = gopacket.RegisterLayerType(
		1014,
		gopacket.LayerTypeMetadata{
			Name: "Get System GUID Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &GetSystemGUIDRsp{}
			}),
		},
	)
	LayerTypeGetDeviceIDRsp = gopacket.RegisterLayerType(
		1015,
		gopacket.LayerTypeMetadata{
			Name: "Get Device ID Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &GetDeviceIDRsp{}
			}),
		},
	)
	LayerTypeGetChassisStatusRsp = gopacket.RegisterLayerType(
		1016,
		gopacket.LayerTypeMetadata{
			Name: "Get Chassis Status Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &GetChassisStatusRsp{}
			}),
		},
	)
	LayerTypeChassisControlReq = gopacket.RegisterLayerType(
		1017,
		gopacket.LayerTypeMetadata{
			Name: "Chassis Control Request",
		},
	)
	LayerTypeGetSDRRepositoryInfoRsp = gopacket.RegisterLayerType(
		1018,
		gopacket.LayerTypeMetadata{
			Name: "Get SDR Repository Info Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &GetSDRRepositoryInfoRsp{}
			}),
		},
	)
	LayerTypeGetSDRReq = gopacket.RegisterLayerType(
		1019,
		gopacket.LayerTypeMetadata{
			Name: "Get SDR Request",
		},
	)
	LayerTypeGetSDRRsp = gopacket.RegisterLayerType(
		1020,
		gopacket.LayerTypeMetadata{
			Name: "Get SDR Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &GetSDRRsp{}
			}),
		},
	)
	LayerTypeSDR = gopacket.RegisterLayerType(
		1021,
		gopacket.LayerTypeMetadata{
			Name: "SDR Header",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &SDR{}
			}),
		},
	)
	LayerTypeFullSensorRecord = gopacket.RegisterLayerType(
		1022,
		gopacket.LayerTypeMetadata{
			Name: "Full Sensor Record",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &FullSensorRecord{}
			}),
		},
	)
	LayerTypeGetSensorReadingReq = gopacket.RegisterLayerType(
		1023,
		gopacket.LayerTypeMetadata{
			Name: "Get Sensor Reading Request",
		},
	)
	LayerTypeGetSensorReadingRsp = gopacket.RegisterLayerType(
		1024,
		gopacket.LayerTypeMetadata{
			Name: "Get Sensor Reading Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &GetSensorReadingRsp{}
			}),
		},
	)
	LayerTypeGetSessionInfoReq = gopacket.RegisterLayerType(
		1025,
		gopacket.LayerTypeMetadata{
			Name: "Get Session Info Request",
		},
	)
	LayerTypeGetSessionInfoRsp = gopacket.RegisterLayerType(
		1026,
		gopacket.LayerTypeMetadata{
			Name: "Get Session Info Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &GetSessionInfoRsp{}
			}),
		},
	)
	LayerTypeSetSessionPrivilegeLevelReq = gopacket.RegisterLayerType(
		1027,
		gopacket.LayerTypeMetadata{
			Name: "Set Session Privilege Level Request",
		},
	)
	LayerTypeSetSessionPrivilegeLevelRsp = gopacket.RegisterLayerType(
		1028,
		gopacket.LayerTypeMetadata{
			Name: "Set Session Privilege Level Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &SetSessionPrivilegeLevelRsp{}
			}),
		},
	)
	LayerTypeGetChannelCipherSuitesReq = gopacket.RegisterLayerType(
		1029,
		gopacket.LayerTypeMetadata{
			Name: "Get Channel Cipher Suites Request",
		},
	)
	LayerTypeGetChannelCipherSuitesRsp = gopacket.RegisterLayerType(
		1030,
		gopacket.LayerTypeMetadata{
			Name: "Get Channel Cipher Suites Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &GetChannelCipherSuitesRsp{}
			}),
		},
	)
	LayerTypeReserveSDRRepositoryRsp = gopacket.RegisterLayerType(
		1031,
		gopacket.LayerTypeMetadata{
			Name: "Reserve SDR Repository Response",
			Decoder: layerexts.BuildDecoder(func() layerexts.LayerDecodingLayer {
				return &ReserveSDRRepositoryRsp{}
			}),
		},
	)
)
