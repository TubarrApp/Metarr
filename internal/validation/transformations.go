package validation

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedparsing"
	"github.com/TubarrApp/gocommon/sharedvalidation"
)

// ValidateAndSetMetaOps parses the meta transformation operations.
func ValidateAndSetMetaOps(metaOpsInput []string) error {
	logger.Pl.D(2, "Validating meta operations...")
	if len(metaOpsInput) == 0 {
		return nil
	}

	parsed, warnings, err := sharedparsing.ParseMetaOps(metaOpsInput)
	for _, w := range warnings {
		logger.Pl.W("%s", w)
	}
	if err != nil {
		return err
	}
	// Metarr is a one-shot run over real files, so a malformed operation stops it rather
	// than quietly applying a partial set. Duplicates are dropped without complaint.
	if invalid := sharedparsing.InvalidEntries(warnings); len(invalid) > 0 {
		return fmt.Errorf("invalid meta operations: %v", invalid)
	}
	if err := sharedvalidation.ValidateMetaOps(parsed); err != nil {
		return err
	}

	ops, err := models.MetaOpsFromShared(parsed)
	if err != nil {
		return err
	}
	logger.Pl.I("Added %d meta operations: %v", len(parsed), sharedparsing.FormatMetaOps(parsed, "", false))

	// Set values into Viper. The flat form is kept too, since channel-scoped operations
	// can only be resolved per file, once that file's URLs are known.
	abstractions.Set(keys.MetaOpsModels, ops)
	abstractions.Set(keys.MetaOpsFlat, parsed)
	return nil
}

// ValidateAndSetFilenameOps checks and validates filename operations.
func ValidateAndSetFilenameOps(filenameOpsInput []string) error {
	if len(filenameOpsInput) == 0 {
		logger.Pl.D(2, "No filename operations to add.")
		return nil
	}

	parsed, warnings, err := sharedparsing.ParseFilenameOps(filenameOpsInput)
	for _, w := range warnings {
		logger.Pl.W("%s", w)
	}
	if err != nil {
		return err
	}
	if invalid := sharedparsing.InvalidEntries(warnings); len(invalid) > 0 {
		return fmt.Errorf("invalid filename operations: %v", invalid)
	}
	if err := sharedvalidation.ValidateFilenameOps(parsed); err != nil {
		return err
	}

	fOpModel, err := models.FilenameOpsFromShared(parsed)
	if err != nil {
		return err
	}
	logger.Pl.I("Added %d filename operations: %v", len(parsed), sharedparsing.FormatFilenameOps(parsed, "", false))

	// Set values into Viper. The flat form is kept too, since channel-scoped operations
	// can only be resolved per file, once that file's URLs are known.
	abstractions.Set(keys.FilenameOpsModels, fOpModel)
	abstractions.Set(keys.FilenameOpsFlat, parsed)
	return nil
}

// ValidateAndSetFilteredMetaOps parses meta operations gated behind a filter.
//
// The filters are evaluated per file once its metadata is read, so this only checks the
// entries are well formed.
func ValidateAndSetFilteredMetaOps(input []string) error {
	if len(input) == 0 {
		return nil
	}

	parsed, warnings, err := sharedparsing.ParseFilteredMetaOps(input)
	for _, w := range warnings {
		logger.Pl.W("%s", w)
	}
	if err != nil {
		return err
	}
	if invalid := sharedparsing.InvalidEntries(warnings); len(invalid) > 0 {
		return fmt.Errorf("invalid filtered meta operations: %v", invalid)
	}

	if err := sharedvalidation.ValidateFilteredMetaOps(parsed); err != nil {
		return err
	}
	logger.Pl.I("Added %d filtered meta operations", len(parsed))

	abstractions.Set(keys.FilteredMetaOpsModels, parsed)
	return nil
}

// ValidateAndSetFilteredFilenameOps parses filename operations gated behind a filter.
//
// The filters read metadata, so they are evaluated per file once its metadata is read;
// this only checks the entries are well formed.
func ValidateAndSetFilteredFilenameOps(input []string) error {
	if len(input) == 0 {
		return nil
	}

	parsed, warnings, err := sharedparsing.ParseFilteredFilenameOps(input)
	for _, w := range warnings {
		logger.Pl.W("%s", w)
	}
	if err != nil {
		return err
	}
	if invalid := sharedparsing.InvalidEntries(warnings); len(invalid) > 0 {
		return fmt.Errorf("invalid filtered filename operations: %v", invalid)
	}

	if err := sharedvalidation.ValidateFilteredFilenameOps(parsed); err != nil {
		return err
	}
	logger.Pl.I("Added %d filtered filename operations", len(parsed))

	abstractions.Set(keys.FilteredFilenameOpsModels, parsed)
	return nil
}
