package enums

import "fmt"

// ProductCategory represents the canonical product categories supported by the catalog.
type ProductCategory string

const (
	ProductCategoryFlower      ProductCategory = "flower"
	ProductCategoryCart        ProductCategory = "cart"
	ProductCategoryPreRoll     ProductCategory = "pre_roll"
	ProductCategoryEdible      ProductCategory = "edible"
	ProductCategoryConcentrate ProductCategory = "concentrate"
	ProductCategoryBeverage    ProductCategory = "beverage"
	ProductCategoryVape        ProductCategory = "vape"
	ProductCategoryTopical     ProductCategory = "topical"
	ProductCategoryTincture    ProductCategory = "tincture"
	ProductCategorySeed        ProductCategory = "seed"
	ProductCategorySeedling    ProductCategory = "seedling"
	ProductCategoryAccessory   ProductCategory = "accessory"
)

var validProductCategories = []ProductCategory{
	ProductCategoryFlower,
	ProductCategoryCart,
	ProductCategoryPreRoll,
	ProductCategoryEdible,
	ProductCategoryConcentrate,
	ProductCategoryBeverage,
	ProductCategoryVape,
	ProductCategoryTopical,
	ProductCategoryTincture,
	ProductCategorySeed,
	ProductCategorySeedling,
	ProductCategoryAccessory,
}

// String implements fmt.Stringer.
func (c ProductCategory) String() string {
	return string(c)
}

// IsValid reports whether the value is a known ProductCategory.
func (c ProductCategory) IsValid() bool {
	for _, candidate := range validProductCategories {
		if candidate == c {
			return true
		}
	}
	return false
}

// ParseProductCategory converts raw input into a ProductCategory.
func ParseProductCategory(value string) (ProductCategory, error) {
	for _, candidate := range validProductCategories {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid product category %q", value)
}

// ProductClassification represents the canonical strain classification values.
type ProductClassification string

const (
	ProductClassificationSativa   ProductClassification = "sativa"
	ProductClassificationHybrid   ProductClassification = "hybrid"
	ProductClassificationIndica   ProductClassification = "indica"
	ProductClassificationCBD      ProductClassification = "cbd"
	ProductClassificationHemp     ProductClassification = "hemp"
	ProductClassificationBalanced ProductClassification = "balanced"
)

var validProductClassifications = []ProductClassification{
	ProductClassificationSativa,
	ProductClassificationHybrid,
	ProductClassificationIndica,
	ProductClassificationCBD,
	ProductClassificationHemp,
	ProductClassificationBalanced,
}

// String implements fmt.Stringer.
func (c ProductClassification) String() string {
	return string(c)
}

// IsValid reports whether the value matches a known ProductClassification.
func (c ProductClassification) IsValid() bool {
	for _, candidate := range validProductClassifications {
		if candidate == c {
			return true
		}
	}
	return false
}

// ParseProductClassification converts raw input into a ProductClassification.
func ParseProductClassification(value string) (ProductClassification, error) {
	for _, candidate := range validProductClassifications {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid product classification %q", value)
}

// ProductUnit defines the available unit types for pricing.
type ProductUnit string

const (
	ProductUnitUnit      ProductUnit = "unit"
	ProductUnitGram      ProductUnit = "gram"
	ProductUnitOunce     ProductUnit = "ounce"
	ProductUnitPound     ProductUnit = "pound"
	ProductUnitEighth    ProductUnit = "eighth"
	ProductUnitSixteenth ProductUnit = "sixteenth"
)

var validProductUnits = []ProductUnit{
	ProductUnitUnit,
	ProductUnitGram,
	ProductUnitOunce,
	ProductUnitPound,
	ProductUnitEighth,
	ProductUnitSixteenth,
}

// String implements fmt.Stringer.
func (u ProductUnit) String() string {
	return string(u)
}

// IsValid reports whether the value matches a known ProductUnit.
func (u ProductUnit) IsValid() bool {
	for _, candidate := range validProductUnits {
		if candidate == u {
			return true
		}
	}
	return false
}

// ParseProductUnit converts raw input into a ProductUnit.
func ParseProductUnit(value string) (ProductUnit, error) {
	for _, candidate := range validProductUnits {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid product unit %q", value)
}

// ProductFlavor represents the canonical flavor tags.
type ProductFlavor string

const (
	ProductFlavorEarthy ProductFlavor = "earthy"
	ProductFlavorCitrus ProductFlavor = "citrus"
	ProductFlavorFruity ProductFlavor = "fruity"
	ProductFlavorFloral ProductFlavor = "floral"
	ProductFlavorCheese ProductFlavor = "cheese"
	ProductFlavorDiesel ProductFlavor = "diesel"
	ProductFlavorSpicy  ProductFlavor = "spicy"
	ProductFlavorSweet  ProductFlavor = "sweet"
	ProductFlavorPine   ProductFlavor = "pine"
	ProductFlavorHerbal ProductFlavor = "herbal"
)

var validProductFlavors = []ProductFlavor{
	ProductFlavorEarthy,
	ProductFlavorCitrus,
	ProductFlavorFruity,
	ProductFlavorFloral,
	ProductFlavorCheese,
	ProductFlavorDiesel,
	ProductFlavorSpicy,
	ProductFlavorSweet,
	ProductFlavorPine,
	ProductFlavorHerbal,
}

// String implements fmt.Stringer.
func (f ProductFlavor) String() string {
	return string(f)
}

// IsValid reports whether the value matches a known ProductFlavor.
func (f ProductFlavor) IsValid() bool {
	for _, candidate := range validProductFlavors {
		if candidate == f {
			return true
		}
	}
	return false
}

// ParseProductFlavor converts raw input into a ProductFlavor.
func ParseProductFlavor(value string) (ProductFlavor, error) {
	for _, candidate := range validProductFlavors {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid product flavor %q", value)
}

// ProductFeeling captures the strain feelings enum.
type ProductFeeling string

const (
	ProductFeelingRelaxed   ProductFeeling = "relaxed"
	ProductFeelingHappy     ProductFeeling = "happy"
	ProductFeelingEuphoric  ProductFeeling = "euphoric"
	ProductFeelingFocused   ProductFeeling = "focused"
	ProductFeelingHungry    ProductFeeling = "hungry"
	ProductFeelingTalkative ProductFeeling = "talkative"
	ProductFeelingCreative  ProductFeeling = "creative"
	ProductFeelingSleepy    ProductFeeling = "sleepy"
	ProductFeelingUplifted  ProductFeeling = "uplifted"
	ProductFeelingCalm      ProductFeeling = "calm"
)

var validProductFeelings = []ProductFeeling{
	ProductFeelingRelaxed,
	ProductFeelingHappy,
	ProductFeelingEuphoric,
	ProductFeelingFocused,
	ProductFeelingHungry,
	ProductFeelingTalkative,
	ProductFeelingCreative,
	ProductFeelingSleepy,
	ProductFeelingUplifted,
	ProductFeelingCalm,
}

// String implements fmt.Stringer.
func (f ProductFeeling) String() string {
	return string(f)
}

// IsValid reports whether the value matches a known ProductFeeling.
func (f ProductFeeling) IsValid() bool {
	for _, candidate := range validProductFeelings {
		if candidate == f {
			return true
		}
	}
	return false
}

// ParseProductFeeling converts raw input into a ProductFeeling.
func ParseProductFeeling(value string) (ProductFeeling, error) {
	for _, candidate := range validProductFeelings {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid product feeling %q", value)
}

// ProductUsage describes the common use cases for a product.
type ProductUsage string

const (
	ProductUsageStressRelief        ProductUsage = "stress_relief"
	ProductUsagePainRelief          ProductUsage = "pain_relief"
	ProductUsageSleep               ProductUsage = "sleep"
	ProductUsageDepression          ProductUsage = "depression"
	ProductUsageMuscleRelaxant      ProductUsage = "muscle_relaxant"
	ProductUsageNausea              ProductUsage = "nausea"
	ProductUsageAnxiety             ProductUsage = "anxiety"
	ProductUsageAppetiteStimulation ProductUsage = "appetite_stimulation"
)

var validProductUsages = []ProductUsage{
	ProductUsageStressRelief,
	ProductUsagePainRelief,
	ProductUsageSleep,
	ProductUsageDepression,
	ProductUsageMuscleRelaxant,
	ProductUsageNausea,
	ProductUsageAnxiety,
	ProductUsageAppetiteStimulation,
}

// String implements fmt.Stringer.
func (u ProductUsage) String() string {
	return string(u)
}

// IsValid reports whether the value matches a known ProductUsage.
func (u ProductUsage) IsValid() bool {
	for _, candidate := range validProductUsages {
		if candidate == u {
			return true
		}
	}
	return false
}

// ParseProductUsage converts raw input into a ProductUsage.
func ParseProductUsage(value string) (ProductUsage, error) {
	for _, candidate := range validProductUsages {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid product usage %q", value)
}
