package media

import (
	"fmt"
	"mime"
	"sort"
	"strings"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

type mimeGroup string

const (
	mimeGroupImages mimeGroup = "images"
	mimeGroupVideos mimeGroup = "videos"
	mimeGroupPDFs   mimeGroup = "pdfs"
)

var mimeGroupNames = map[mimeGroup]string{
	mimeGroupImages: "images",
	mimeGroupVideos: "videos",
	mimeGroupPDFs:   "PDFs",
}

var mimeGroupTypes = map[mimeGroup][]string{
	mimeGroupImages: {"image/png", "image/jpeg", "image/webp", "image/gif"},
	mimeGroupVideos: {"video/mp4", "video/webm"},
	mimeGroupPDFs:   {"application/pdf"},
}

var allowedMimeGroupsByKind = map[enums.MediaKind][]mimeGroup{
	enums.MediaKindProduct:    {mimeGroupImages, mimeGroupVideos},
	enums.MediaKindAds:        {mimeGroupImages, mimeGroupVideos},
	enums.MediaKindPDF:        {mimeGroupPDFs},
	enums.MediaKindLicenseDoc: {mimeGroupPDFs, mimeGroupImages},
	enums.MediaKindCOA:        {mimeGroupPDFs},
	enums.MediaKindManifest:   {mimeGroupPDFs},
	enums.MediaKindUser:       {mimeGroupImages},
	enums.MediaKindStore:      {mimeGroupImages},
	enums.MediaKindOther:      {mimeGroupPDFs, mimeGroupImages, mimeGroupVideos},
}

var (
	mimeTypesByKind        = buildMimeTypesByKind()
	mimeDescriptionsByKind = buildMimeDescriptions()
)

func buildMimeTypesByKind() map[enums.MediaKind][]string {
	result := make(map[enums.MediaKind][]string, len(allowedMimeGroupsByKind))
	for kind, groups := range allowedMimeGroupsByKind {
		set := make(map[string]struct{})
		for _, group := range groups {
			for _, value := range mimeGroupTypes[group] {
				set[value] = struct{}{}
			}
		}
		list := make([]string, 0, len(set))
		for value := range set {
			list = append(list, value)
		}
		sort.Strings(list)
		result[kind] = list
	}
	return result
}

func buildMimeDescriptions() map[enums.MediaKind]string {
	result := make(map[enums.MediaKind]string, len(allowedMimeGroupsByKind))
	for kind, groups := range allowedMimeGroupsByKind {
		var descriptions []string
		for _, group := range groups {
			if name, ok := mimeGroupNames[group]; ok {
				descriptions = append(descriptions, name)
			}
		}
		result[kind] = humanReadableList(descriptions)
	}
	return result
}

func humanReadableList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return fmt.Sprintf("%s or %s", items[0], items[1])
	default:
		return fmt.Sprintf("%s, or %s", strings.Join(items[:len(items)-1], ", "), items[len(items)-1])
	}
}

func sniffMimeType(value string) (string, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return "", fmt.Errorf("mime type required")
	}
	mediaType, _, err := mime.ParseMediaType(clean)
	if err != nil {
		return "", fmt.Errorf("mime type invalid: %w", err)
	}
	if mediaType == "" {
		return "", fmt.Errorf("mime type missing")
	}
	return strings.ToLower(mediaType), nil
}

func allowedMimeDescription(kind enums.MediaKind) string {
	if msg, ok := mimeDescriptionsByKind[kind]; ok && msg != "" {
		return msg
	}
	return "the approved mime types"
}
