package awsaccounts

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/cryptoenvelope"
	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

type Resolver struct {
	Repository store.Repository
	Box        *cryptoenvelope.Box
	Factory    cloudaws.Factory
}

func (r Resolver) OpenAWS(ctx context.Context, organizationID, accountID uuid.UUID, region string) (cloudaws.Provider, error) {
	record, err := r.Repository.AWSAccountConnection(ctx, organizationID, accountID)
	if err != nil {
		return nil, err
	}
	if region == "" {
		region = record.DefaultRegion
	}
	config := cloudaws.AccountConfig{AccountID: record.AccountID, RoleARN: record.RoleARN, Region: region}
	if len(record.EncryptedExternalID) > 0 {
		if r.Box == nil {
			return nil, errors.New("AWS credential encryption is unavailable")
		}
		externalID, openErr := r.Box.Open(record.EncryptedExternalID, cloudaws.CredentialContext(organizationID, accountID, "external-id"))
		if openErr != nil {
			return nil, errors.New("AWS external ID could not be decrypted")
		}
		config.ExternalID = string(externalID)
		zero(externalID)
	}
	if len(record.EncryptedAccessKeyID) > 0 {
		if r.Box == nil {
			return nil, errors.New("AWS credential encryption is unavailable")
		}
		id, openErr := r.Box.Open(record.EncryptedAccessKeyID, cloudaws.CredentialContext(organizationID, accountID, "access-key-id"))
		if openErr != nil {
			return nil, errors.New("AWS bootstrap credentials could not be decrypted")
		}
		secret, openErr := r.Box.Open(record.EncryptedSecretAccessKey, cloudaws.CredentialContext(organizationID, accountID, "secret-access-key"))
		if openErr != nil {
			zero(id)
			return nil, errors.New("AWS bootstrap credentials could not be decrypted")
		}
		config.AccessKeyID = string(id)
		config.SecretAccessKey = string(secret)
		zero(id)
		zero(secret)
	}
	factory := r.Factory
	if factory == nil {
		factory = cloudaws.SDKFactory{}
	}
	provider, err := factory.Open(ctx, config)
	config.ExternalID = ""
	config.AccessKeyID = ""
	config.SecretAccessKey = ""
	return provider, err
}
func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
