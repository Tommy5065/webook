package alibaba

import (
	"context"
	"encoding/json"
	"webookApp/internal/code"

	openApi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dypnsapi20170525 "github.com/alibabacloud-go/dypnsapi-20170525/v3/client"
	"github.com/alibabacloud-go/tea/tea"
	credentials "github.com/aliyun/credentials-go/credentials"
	"go.uber.org/zap"
)

type AlibabaCloudSMS struct {
	accessType      string
	accessKeyID     string
	accessKeySecret string
	signame         string
}

// NewAlibabaCloud使用alibaba作为短信服务商
func NewAlibabaCloud(accessType, accessKeyID, accessKeySecret, sigName string) *AlibabaCloudSMS {
	return &AlibabaCloudSMS{
		accessType:      accessType,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		signame:         sigName,
	}
}

func (al *AlibabaCloudSMS) Send(ctx context.Context, args []string, phoneNumber string) error {
	client, err := al.create_credential()
	if err != nil {
		zap.L().Error("alibaba credential err:", zap.Error(err))
		return err
	}

	jsonTemplateParam, err := json.Marshal(&code.TemplateParam{
		Code: args[0],
		Min:  args[1],
	})
	if err != nil {
		return err
	}
	sendSmsVerifyCodeRequest := &dypnsapi20170525.SendSmsVerifyCodeRequest{
		SignName:      tea.String(al.signame),
		PhoneNumber:   tea.String(phoneNumber),
		TemplateCode:  tea.String(args[2]),
		TemplateParam: tea.String(string(jsonTemplateParam)),
		ValidTime:     tea.Int64(10),
	}
	_, err = client.SendSmsVerifyCode(sendSmsVerifyCodeRequest)
	if err != nil {
		zap.L().Error("alibaba sendSmsCode err:", zap.Error(err))
		return err
	}
	return nil
}

// create_credential创建alibaba发送短信客户端
func (al *AlibabaCloudSMS) create_credential() (client *dypnsapi20170525.Client, err error) {
	config_ := new(credentials.Config).SetType(al.accessType).SetAccessKeyId(al.accessKeyID).SetAccessKeySecret(al.accessKeySecret)
	credential, err := credentials.NewCredential(config_)
	if err != nil {
		return client, err
	}

	config := &openApi.Config{
		Credential: credential,
		Endpoint:   tea.String("dypnsapi.aliyuncs.com"),
		// ConnectTimeout: tea.Int(100),
	}
	return dypnsapi20170525.NewClient(config)
}
