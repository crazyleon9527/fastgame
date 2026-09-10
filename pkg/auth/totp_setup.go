package auth

import (
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

type TotpSetup struct {
	Secret       string
	Provisioning string
}

func NewTotpSetup(issuer, accountName string) (*TotpSetup, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return nil, err
	}
	return &TotpSetup{
		Secret:       key.Secret(),
		Provisioning: key.URL(),
	}, nil
}
