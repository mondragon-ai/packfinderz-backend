package enums

import "fmt"

// MediaKind defines where the media object is used.
type MediaKind string

const (
	MediaKindProduct    MediaKind = "product"
	MediaKindAds        MediaKind = "ads"
	MediaKindPDF        MediaKind = "pdf"
	MediaKindLicenseDoc MediaKind = "license_doc"
	MediaKindCOA        MediaKind = "coa"
	MediaKindManifest   MediaKind = "manifest"
	MediaKindUser       MediaKind = "user"
	MediaKindOther      MediaKind = "other"
)

var validMediaKinds = []MediaKind{
	MediaKindProduct,
	MediaKindAds,
	MediaKindPDF,
	MediaKindLicenseDoc,
	MediaKindCOA,
	MediaKindManifest,
	MediaKindUser,
	MediaKindOther,
}

// String returns the literal string for the kind.
func (m MediaKind) String() string {
	return string(m)
}

// IsValid reports whether the kind is known.
func (m MediaKind) IsValid() bool {
	for _, candidate := range validMediaKinds {
		if candidate == m {
			return true
		}
	}
	return false
}

// ParseMediaKind converts raw input into a MediaKind.
func ParseMediaKind(value string) (MediaKind, error) {
	for _, candidate := range validMediaKinds {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid media kind %q", value)
}
