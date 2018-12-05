package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/captv/kumi/modules/log"
)

var logger = log.NewLogger("AWSS3")

var svc *s3.S3

func init() {
	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String(endpoints.ApNortheast2RegionID)}))
	svc = s3.New(sess)
}

// ListObjects ...
func ListObjects(bucket string, prefix string) (object []string, err error) {
	/*sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("ap-northeast-2"),
		Credentials: credentials.NewStaticCredentials("AKID", "SECRET_KEY", ""),
	})

	svc := s3.New(sess)*/

	err = svc.ListObjectsPages(&s3.ListObjectsInput{
		Bucket: &bucket,
		Prefix: &prefix,
	}, func(p *s3.ListObjectsOutput, last bool) (shouldContinue bool) {
		for _, obj := range p.Contents {
			object = append(object, *obj.Key)
		}

		return true
	})
	if err != nil {
		return
	}

	return
}
