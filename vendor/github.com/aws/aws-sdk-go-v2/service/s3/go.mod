module github.com/aws/aws-sdk-go-v2/service/s3

go 1.15

require (
	github.com/aws/aws-sdk-go-v2 v1.13.0
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.2.0
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.4
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.2.0
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.7.0
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.7.0
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.11.0
	github.com/aws/smithy-go v1.10.0
	github.com/google/go-cmp v0.5.6
)

replace github.com/aws/aws-sdk-go-v2 => ../../

replace github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream => ../../aws/protocol/eventstream/

replace github.com/aws/aws-sdk-go-v2/internal/configsources => ../../internal/configsources/

replace github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 => ../../internal/endpoints/v2/

replace github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding => ../../service/internal/accept-encoding/

replace github.com/aws/aws-sdk-go-v2/service/internal/presigned-url => ../../service/internal/presigned-url/

replace github.com/aws/aws-sdk-go-v2/service/internal/s3shared => ../../service/internal/s3shared/
