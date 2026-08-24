package domain

import "time"

type ReleaseCredential struct {
	CredentialNumber   string    `json:"credentialNumber"`
	CaseID             string    `json:"caseId"`
	FrozenVersion      int64     `json:"frozenVersion"`
	EvidenceDigest     string    `json:"evidenceDigest"`
	Decision           string    `json:"decision"`
	Issuer             string    `json:"issuer"`
	IssuedAt           time.Time `json:"issuedAt"`
	VerificationDigest string    `json:"verificationDigest"`
}

type credentialMaterial struct {
	CredentialNumber string    `json:"credentialNumber"`
	CaseID           string    `json:"caseId"`
	FrozenVersion    int64     `json:"frozenVersion"`
	EvidenceDigest   string    `json:"evidenceDigest"`
	Decision         string    `json:"decision"`
	Issuer           string    `json:"issuer"`
	IssuedAt         time.Time `json:"issuedAt"`
}

func (c ReleaseCredential) material() credentialMaterial {
	return credentialMaterial{CredentialNumber: c.CredentialNumber, CaseID: c.CaseID, FrozenVersion: c.FrozenVersion, EvidenceDigest: c.EvidenceDigest, Decision: c.Decision, Issuer: c.Issuer, IssuedAt: c.IssuedAt}
}

func (c ReleaseCredential) Verify(snapshot *FrozenSnapshot) bool {
	return snapshot != nil && snapshot.Verify() && snapshot.EvidenceDigest == c.EvidenceDigest && c.FrozenVersion == snapshot.FrozenVersion && VerifyDigest(c.material(), c.VerificationDigest)
}

func (c *InspectionCase) IssueCredential(number, issuer string, frozenVersion int64, now time.Time) (*ReleaseCredential, error) {
	if c.Status != StatusFrozen || c.Frozen == nil {
		return nil, ErrInvalidState
	}
	if c.Credential != nil {
		return nil, ErrAlreadyIssued
	}
	if frozenVersion != c.Frozen.FrozenVersion || c.Version != frozenVersion {
		return nil, ErrConflict
	}
	credential := ReleaseCredential{CredentialNumber: number, CaseID: c.ID, FrozenVersion: frozenVersion, EvidenceDigest: c.Frozen.EvidenceDigest, Decision: "released", Issuer: issuer, IssuedAt: now.UTC()}
	digest, err := StableDigest(credential.material())
	if err != nil {
		return nil, err
	}
	credential.VerificationDigest = digest
	c.Credential = &credential
	c.Status = StatusReleased
	c.Touch(now)
	return &credential, nil
}
