package keygen

import (
	"os"
	"runtime"
	"time"

	"github.com/keygen-sh/jsonapi-go"
)

type SchemeCode string

const (
	SchemeCodeEd25519 SchemeCode = "ED25519_SIGN"
)

// License represents a Keygen license object.
type License struct {
	ID               string                 `json:"-"`
	Type             string                 `json:"-"`
	Name             string                 `json:"name"`
	Key              string                 `json:"key"`
	Expiry           *time.Time             `json:"expiry"`
	Scheme           SchemeCode             `json:"scheme"`
	RequireHeartbeat bool                   `json:"requireHeartbeat"`
	LastValidated    *time.Time             `json:"lastValidated"`
	Created          time.Time              `json:"created"`
	Updated          time.Time              `json:"updated"`
	Metadata         map[string]interface{} `json:"metadata"`
	PolicyId         string                 `json:"-"`
}

// SetID implements the jsonapi.UnmarshalResourceIdentifier interface.
func (l *License) SetID(id string) error {
	l.ID = id
	return nil
}

// SetType implements the jsonapi.UnmarshalResourceIdentifier interface.
func (l *License) SetType(t string) error {
	l.Type = t
	return nil
}

// SetData implements the jsonapi.UnmarshalData interface.
func (l *License) SetData(to func(target interface{}) error) error {
	return to(l)
}

// SetRelationships implements the jsonapi.UnmarshalRelationship interface.
func (l *License) SetRelationships(relationships map[string]interface{}) error {
	if relationship, ok := relationships["policy"]; ok {
		l.PolicyId = relationship.(*jsonapi.ResourceObjectIdentifier).ID
	}

	return nil
}

// Validate performs a license validation, scoped to any provided fingerprints. It
// returns an error if the license is invalid, e.g. ErrLicenseNotActivated,
// ErrLicenseExpired or ErrLicenseTooManyMachines.
func (l *License) Validate(fingerprints ...string) error {
	client := &Client{Account: Account, LicenseKey: LicenseKey, Token: Token, PublicKey: PublicKey, UserAgent: UserAgent}
	params := &validate{fingerprints}
	validation := &validation{}

	if _, err := client.Post("licenses/"+l.ID+"/actions/validate", params, validation); err != nil {
		if _, ok := err.(*NotFoundError); ok {
			return ErrLicenseInvalid
		} else {
			return err
		}
	}

	*l = validation.License

	if validation.Result.Code == ValidationCodeValid {
		return nil
	}

	switch {
	case validation.Result.Code == ValidationCodeFingerprintScopeMismatch ||
		validation.Result.Code == ValidationCodeNoMachines ||
		validation.Result.Code == ValidationCodeNoMachine:
		return ErrLicenseNotActivated
	case validation.Result.Code == ValidationCodeExpired:
		return ErrLicenseExpired
	case validation.Result.Code == ValidationCodeSuspended:
		return ErrLicenseSuspended
	case validation.Result.Code == ValidationCodeTooManyMachines:
		return ErrLicenseTooManyMachines
	case validation.Result.Code == ValidationCodeTooManyCores:
		return ErrLicenseTooManyCores
	case validation.Result.Code == ValidationCodeFingerprintScopeRequired ||
		validation.Result.Code == ValidationCodeFingerprintScopeEmpty:
		return ErrValidationFingerprintMissing
	case validation.Result.Code == ValidationCodeHeartbeatNotStarted:
		return ErrHeartbeatRequired
	case validation.Result.Code == ValidationCodeHeartbeatDead:
		return ErrHeartbeatDead
	case validation.Result.Code == ValidationCodeProductScopeRequired ||
		validation.Result.Code == ValidationCodeProductScopeEmpty:
		return ErrValidationProductMissing
	default:
		return ErrLicenseInvalid
	}
}

// Verify checks if the license's key is genuine by cryptographically verifying the
// key using your PublicKey. If the license is genuine, the decoded dataset from the
// key will be returned. An error will be returned if the license is not genuine, or
// if the key is not signed, e.g. ErrLicenseNotGenuine or ErrLicenseNotSigned.
func (l *License) Verify() ([]byte, error) {
	if l.Scheme == "" {
		return nil, ErrLicenseNotSigned
	}

	verifier := &verifier{PublicKey: PublicKey}

	return verifier.VerifyLicense(l)
}

// Activate performs a machine activation for the license, identified by the provided
// fingerprint. If the activation is successful, the new machine will be returned. An
// error will be returned if the activation fails, e.g. ErrMachineLimitExceeded
// or ErrMachineAlreadyActivated.
func (l *License) Activate(fingerprint string) (*Machine, error) {
	client := &Client{Account: Account, LicenseKey: LicenseKey, Token: Token, PublicKey: PublicKey, UserAgent: UserAgent}
	hostname, _ := os.Hostname()
	params := &Machine{
		Fingerprint: fingerprint,
		Hostname:    hostname,
		Platform:    runtime.GOOS + "/" + runtime.GOARCH,
		Cores:       runtime.NumCPU(),
		LicenseID:   l.ID,
	}

	machine := &Machine{}
	if _, err := client.Post("machines", params, machine); err != nil {
		return nil, err
	}

	return machine, nil
}

// Deactivate performs a machine deactivation, identified by the provided ID. The ID
// can be the machine's UUID or the machine's fingerprint. An error will be returned
// if the machine deactivation fails.
func (l *License) Deactivate(id string) error {
	client := &Client{Account: Account, LicenseKey: LicenseKey, Token: Token, PublicKey: PublicKey, UserAgent: UserAgent}

	_, err := client.Delete("machines/"+id, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

// Machine retreives a machine, identified by the provided ID. The ID can be the machine's
// UUID or the machine's fingerprint. An error will be returned if it does not exist.
func (l *License) Machine(id string) (*Machine, error) {
	client := &Client{Account: Account, LicenseKey: LicenseKey, Token: Token, PublicKey: PublicKey, UserAgent: UserAgent}
	machine := &Machine{}

	if _, err := client.Get("machines/"+id, nil, machine); err != nil {
		return nil, err
	}

	return machine, nil
}

// Machines lists up to 100 machines for the license.
func (l *License) Machines() (Machines, error) {
	client := &Client{Account: Account, LicenseKey: LicenseKey, Token: Token, PublicKey: PublicKey, UserAgent: UserAgent}
	machines := Machines{}

	if _, err := client.Get("licenses/"+l.ID+"/machines", querystring{Limit: 100}, &machines); err != nil {
		return nil, err
	}

	return machines, nil
}

// Machines lists up to 100 entitlements for the license.
func (l *License) Entitlements() (Entitlements, error) {
	client := &Client{Account: Account, LicenseKey: LicenseKey, Token: Token, PublicKey: PublicKey, UserAgent: UserAgent}
	entitlements := Entitlements{}

	if _, err := client.Get("licenses/"+l.ID+"/entitlements", querystring{Limit: 100}, &entitlements); err != nil {
		return nil, err
	}

	return entitlements, nil
}

// Checkout generates an encrypted license file. Returns a LicenseFile.
func (l *License) Checkout() (*LicenseFile, error) {
	client := &Client{Account: Account, LicenseKey: LicenseKey, Token: Token, PublicKey: PublicKey, UserAgent: UserAgent}
	lic := &LicenseFile{}

	if _, err := client.Post("licenses/"+l.ID+"/actions/check-out", querystring{Encrypt: true, Include: "entitlements"}, lic); err != nil {
		return nil, err
	}

	return lic, nil
}
