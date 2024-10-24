package metadata

import (
	"Metarr/internal/backup"
	"Metarr/internal/browser"
	"Metarr/internal/cmd"
	"Metarr/internal/consts"
	"Metarr/internal/enums"
	"Metarr/internal/keys"
	"Metarr/internal/logging"
	"Metarr/internal/models"
	"Metarr/internal/naming"
	"Metarr/internal/shared"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

var (
	muPrint   sync.Mutex
	muProcess sync.Mutex
	muMetaAdd sync.Mutex
	tryURL    []string
)

// ProcessJSONFile reads a single JSON file and fills in the metadata
func ProcessJSONFile(m *models.FileData) (*models.FileData, error) {

	// Function mutex
	muProcess.Lock()
	defer muProcess.Unlock()

	filePath := m.JSONFilePath

	// Open the file
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	var metadata map[string]interface{}
	err = json.Unmarshal(fileContent, &metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Return early if user wants to skip videos
	if cmd.GetBool(keys.SkipVideos) {
		return m, nil
	}

	// Make metadata adjustments per user selection
	err = naming.MakeMetaEdits(fileContent, metadata, file, m)
	if err != nil {
		return nil, err
	}

	// Seek to the beginning before reading
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	if !fillMetaFields(m, metadata, file) {
		logging.PrintD(2, "Some metafields were unfilled")
	}

	// Make date tag
	logging.PrintD(3, "About to make date tag for: %v", file.Name())
	if cmd.Get(keys.FileDateFmt).(enums.FilenameDateFormat) != enums.FILEDATE_SKIP {
		m.FilenameDateTag, err = makeDateTag(metadata, file.Name())
		if err != nil {
			logging.PrintE(0, "Failed to make date tag: %v", err)
		}
	}

	// Add new filename tag for files
	logging.PrintD(3, "About to make prefix tag for: %v", file.Name())
	m.FilenameMetaPrefix = makeFilenameTag(metadata, file)

	return m, nil
}

// Primary function to fill out meta fields before writing
func fillMetaFields(m *models.FileData, metadata map[string]interface{}, file *os.File) bool {

	allFilled := true

	if !fillWebpageDetails(m, metadata) {
		logging.PrintI("No URL metadata found")
		allFilled = false
	} else if err := insertScrapedMetadata(m, file, enums.WEBCLASS_WEBINFO); err != nil {
		logging.PrintI("Inserted scraped webpage metadata for file '%s'", m.OriginalVideoBaseName)
	}

	if !fillTitles(m, metadata) {
		logging.PrintI("No title metadata found")
		allFilled = false
	}

	if !fillDescriptions(m, metadata, file) {
		logging.PrintI("No description metadata found")
		allFilled = false
	} else if err := insertScrapedMetadata(m, file, enums.WEBCLASS_DESCRIPTION); err != nil {
		logging.PrintI("Inserted scraped description metadata for file '%s'", m.OriginalVideoBaseName)
	}

	if !fillCredits(m, metadata, file) {
		logging.PrintI("No credits metadata found")
		allFilled = false
	} else if err := insertScrapedMetadata(m, file, enums.WEBCLASS_CREDITS); err != nil {
		logging.PrintI("Inserted scraped credits metadata for file '%s'", m.OriginalVideoBaseName)
	}

	if fillTimestamps(m, metadata, file) {
		m.MDates.FormattedDate = formatDate(m)
		if err := insertScrapedMetadata(m, file, enums.WEBCLASS_DATE); err != nil {
			logging.PrintI("Inserted scraped date & time metadata for file '%s'", m.OriginalVideoBaseName)
		}
	} else {
		logging.PrintI("No date metadata found")
		allFilled = false
	}

	return allFilled
}

// Grabs details necessary to scrape the web for missing metafields
func fillWebpageDetails(m *models.FileData, metadata map[string]interface{}) bool {

	w := m.MWebData

	printMap := make(map[string]string)
	priorityMap := []string{consts.JWebpageURL, consts.JURL, consts.JReferer, consts.JWebpageDomain, consts.JDomain}

	var isFilled bool

	for _, wanted := range priorityMap {
		for key, value := range metadata {

			if val, ok := value.(string); ok && val != "" {
				if key == wanted {
					switch {
					case key == consts.JWebpageURL && w.WebpageURL == "":

						w.WebpageURL = val
						printMap[key] = val
						tryURL = append(tryURL, val)

						isFilled = true

					case key == consts.JURL && w.VideoURL == "":

						w.VideoURL = val
						printMap[key] = val
						tryURL = append(tryURL, val)

						isFilled = true

					case key == consts.JReferer && w.Referer == "":

						w.Referer = val
						printMap[key] = val
						tryURL = append(tryURL, val)

						isFilled = true

					case key == consts.JWebpageDomain && w.Domain == "":

						w.Domain = val
						printMap[key] = val

						isFilled = true

					case key == consts.JDomain && w.Domain == "":

						w.Domain = val
						printMap[key] = val

						isFilled = true
					}
				}
			}
		}
	}

	logging.PrintD(2, "Stored URLs for scraping missing fields: %v", tryURL)

	printGrabbedFields("web details", &printMap)

	return isFilled
}

// fillTitles grabs the fulltitle ("title")
func fillTitles(m *models.FileData, metadata map[string]interface{}) bool {

	printMap := make(map[string]string)

	for key, value := range metadata {
		if val, ok := value.(string); ok && val != "" {

			switch {
			case key == consts.JTitle:
				m.MTitleDesc.Title = val
				printMap[key] = val

			case key == consts.JFallbackTitle:
				m.MTitleDesc.FallbackTitle = val
				printMap[key] = val

			case key == consts.JSubtitle:
				m.MTitleDesc.Subtitle = val
				printMap[key] = val

			}
		}
	}

	if m.MTitleDesc.Title == "" && m.MTitleDesc.FallbackTitle != "" {
		m.MTitleDesc.Title = m.MTitleDesc.FallbackTitle
	}

	printGrabbedFields("title", &printMap)

	return m.MTitleDesc.Title != ""
}

// fillCredits fills in the metadator for credits (e.g. actor, director, uploader)
func fillCredits(m *models.FileData, metadata map[string]interface{}, file *os.File) bool {

	c := m.MCredits
	w := m.MWebData

	fieldMap := map[string]*string{
		// Order by importance
		consts.JCreator:   &c.Creator,
		consts.JPerformer: &c.Performer,
		consts.JAuthor:    &c.Author,
		consts.JArtist:    &c.Artist, // May be alias for "author" in some systems
		consts.JDirector:  &c.Director,
		consts.JActor:     &c.Actor,
		consts.JStudio:    &c.Studio,
		consts.JProducer:  &c.Producer,

		consts.JUploader: &c.Uploader,
		"uploaded_by":    &c.Uploader, // Try alias

		consts.JPublisher:  &c.Publisher,
		"publishing_house": &c.Publisher, // Try alias

		consts.JComposer: &c.Composer,
		"songwriter":     &c.Composer,
	}

	dataFilled := unpackJSON("credits", fieldMap, metadata)

	switch {
	case dataFilled:
		return true
	case w.WebpageURL == "":
		logging.PrintI("Page URL not found in metadata, so cannot scrape for missing credits in '%s'", m.JSONFilePath)
		return false
	}

	var err error
	w.Cookies, err = browser.GetBrowserCookies(w.WebpageURL)
	if err != nil {
		logging.PrintE(2, "Was unable to grab browser cookies: %v", err)
	}

	var credits string

	for _, try := range tryURL {
		credits, err = browser.ScrapeForMetadata(try, w.Cookies, enums.WEBCLASS_CREDITS)
		if err != nil {
			logging.PrintE(0, "Failed to scrape '%s' for credits: %v", try, err)
		} else {
			break
		}
	}

	if credits != "" {

		for _, value := range fieldMap {
			if *value == "" {
				*value = credits
			}
		}

		err := insertScrapedMetadata(m, file, enums.WEBCLASS_CREDITS)
		if err != nil {
			logging.PrintE(0, "Failed to insert new metadata (%s) into JSON file '%s': %v", credits, m.JSONFilePath, err)
		}

		return true
	} else {
		return false
	}
}

// fillDescriptions grabs description metadata from JSON
func fillDescriptions(m *models.FileData, metadata map[string]interface{}, file *os.File) bool {

	d := m.MTitleDesc
	w := m.MWebData

	fieldMap := map[string]*string{
		// Order by importance
		consts.JLongDescription:  &d.LongDescription,
		consts.JLong_Description: &d.Long_Description,
		consts.JDescription:      &d.Description,
		consts.JSynopsis:         &d.Synopsis,
		consts.JSummary:          &d.Summary,
		consts.JComment:          &d.Comment,
	}

	dataFilled := unpackJSON("descriptions", fieldMap, metadata)

	switch {
	case dataFilled:
		return true
	case w.WebpageURL == "":
		logging.PrintI("Page URL not found in metadata, so cannot scrape for missing description in '%s'", m.JSONFilePath)
		return false
	}

	var err error
	w.Cookies, err = browser.GetBrowserCookies(w.WebpageURL)
	if err != nil {
		logging.PrintE(2, "Was unable to grab browser cookies: %v", err)
	}

	var description string
	for _, try := range tryURL {
		description, err = browser.ScrapeForMetadata(try, w.Cookies, enums.WEBCLASS_DESCRIPTION)
		if err != nil {
			logging.PrintE(0, "Failed to scrape '%s' for credits: %v", try, err)
		} else {
			break
		}
	}

	// Infer remaining fields from description
	if description != "" {
		for _, value := range fieldMap {
			if *value == "" {
				*value = description
			}
		}

		// Insert new scraped fields into file
		err := insertScrapedMetadata(m, file, enums.WEBCLASS_DESCRIPTION)
		if err != nil {
			logging.PrintE(0, "Failed to insert new metadata (%s) into JSON file '%s': %v", description, m.JSONFilePath, err)
		}
		return true
	} else {
		return false
	}
}

// fillTimestamps grabs timestamp metadata from JSON
func fillTimestamps(m *models.FileData, metadata map[string]interface{}, file *os.File) bool {

	t := m.MDates
	w := m.MWebData

	printMap := make(map[string]string)

	fieldMap := map[string]*string{
		// Order by importance
		consts.JReleaseDate: &t.ReleaseDate,
		"releasedate":       &t.ReleaseDate, // Try alias
		"released_on":       &t.ReleaseDate, // Try alias

		consts.JOriginallyAvailable: &t.Originally_Available_At,
		"originally_available":      &t.Originally_Available_At, // Try alias
		"originallyavailable":       &t.Originally_Available_At, // Try alias

		consts.JDate: &t.Date,

		consts.JUploadDate: &t.UploadDate,
		"uploaddate":       &t.UploadDate, // Try alias
		"uploaded_on":      &t.UploadDate, // Try alias

		consts.JReleaseYear: &t.Year,
		consts.JYear:        &t.Year,

		consts.JCreationTime: &t.Creation_Time,
		"created_at":         &t.Creation_Time, // Try alias
	}

	for key, value := range metadata {
		if strVal, ok := value.(string); ok {
			if _, exists := fieldMap[key]; exists {
				*fieldMap[key] = strVal
				if printMap[key] == "" {
					printMap[key] = strVal
				}
			}
		}
	}

	var gotRelevantDate bool

	switch {
	case t.Originally_Available_At != "" && len(t.Originally_Available_At) >= 6:

		gotRelevantDate = true

		if t.Creation_Time == "" {
			t.Creation_Time = t.Originally_Available_At + "T00:00:00Z"

		}

	case t.ReleaseDate != "" && len(t.ReleaseDate) >= 6:

		gotRelevantDate = true

		if t.Creation_Time == "" {
			t.Creation_Time = t.ReleaseDate + "T00:00:00Z"

		}
		if t.Originally_Available_At == "" {
			t.Originally_Available_At = t.ReleaseDate
		}

	case t.Date != "" && len(t.Date) >= 6:

		gotRelevantDate = true

		if t.Creation_Time == "" {
			t.Creation_Time = t.Date + "T00:00:00Z"

		}
		if t.Originally_Available_At == "" {
			t.Originally_Available_At = t.Date
		}

	case t.UploadDate != "" && len(t.UploadDate) >= 6:
		if t.Creation_Time == "" {
			t.Creation_Time = t.UploadDate + "T00:00:00Z"

		}
		if t.Originally_Available_At == "" {
			t.Originally_Available_At = t.UploadDate
		}
	}

	if t.Date == "" {
		switch {
		case t.ReleaseDate != "":
			t.Date = t.ReleaseDate
			t.Originally_Available_At = t.ReleaseDate

		case t.UploadDate != "":
			t.Date = t.UploadDate
			t.Originally_Available_At = t.UploadDate
		}
	}

	if t.Year == "" {
		switch {
		case t.Date != "" && len(t.Date) >= 4:
			t.Year = t.Date[:4]
		case t.UploadDate != "" && len(t.UploadDate) >= 4:
			t.Year = t.UploadDate[:4]
		}
	}

	printGrabbedFields("time and date", &printMap)

	switch {
	case gotRelevantDate:
		logging.PrintD(3, "Got a relevant date, proceeding...")
		return true

	case w.WebpageURL == "":
		logging.PrintI("Page URL not found in metadata, so cannot scrape for missing date in '%s'", m.JSONFilePath)
		return false
	}

	var err error
	w.Cookies, err = browser.GetBrowserCookies(w.WebpageURL)
	if err != nil {
		logging.PrintE(2, "Was unable to grab browser cookies: %v", err)
	}

	var scrapedDate string
	for _, try := range tryURL {
		logging.PrintD(3, "Scouring URL '%s' for missing dates", try)
		scrapedDate, err = browser.ScrapeForMetadata(try, w.Cookies, enums.WEBCLASS_DATE)
		if err != nil {
			logging.PrintE(0, "Failed to scrape '%s' for credits: %v", try, err)
		} else {
			break
		}
	}

	var date string
	if scrapedDate != "" {
		date, err = naming.ParseAndFormatDate(scrapedDate)
		if err != nil || date == "" {
			logging.PrintE(0, "Failed to parse date '%s': %v", scrapedDate, err)
		} else {

			t.ReleaseDate = date
			t.Date = date
			t.Creation_Time = date + "T00:00:00Z"

			if len(date) >= 4 {
				t.Year = date[:4]
			}

			printMap[consts.JReleaseDate] = t.ReleaseDate
			printMap[consts.JDate] = t.Date
			printMap[consts.JYear] = t.Year

			printGrabbedFields("time and date", &printMap)

			err := insertScrapedMetadata(m, file, enums.WEBCLASS_DATE)
			if err != nil {
				logging.PrintE(0, "Failed to insert new metadata (%s) into JSON file '%s': %v", date, m.JSONFilePath, err)
			}
			return true
		}
	}
	return false
}

// formatDate formats timestamps into a hyphenated form
func formatDate(m *models.FileData) string {

	var result string = ""
	var ok bool = false

	d := m.MDates

	if !ok && d.Originally_Available_At != "" {

		logging.PrintD(2, "Attempting to format originally available date: %v", d.Originally_Available_At)
		result, ok = yyyyMmDd(d.Originally_Available_At)
	}

	if !ok && d.ReleaseDate != "" {

		logging.PrintD(2, "Attempting to format release date: %v", d.ReleaseDate)
		result, ok = yyyyMmDd(d.ReleaseDate)
	}

	if !ok && d.Date != "" {

		logging.PrintD(2, "Attempting to format date: %v", d.Date)
		result, ok = yyyyMmDd(d.Date)
	}

	if !ok && d.UploadDate != "" {

		logging.PrintD(2, "Attempting to format upload date: %v", d.UploadDate)
		result, ok = yyyyMmDd(d.UploadDate)
	}

	if !ok && d.Creation_Time != "" {

		logging.PrintD(3, "Attempting to format creation time: %v", d.Creation_Time)
		result, ok = yyyyMmDd(d.Creation_Time)
	}

	if !ok {
		logging.PrintE(0, "Failed to format dates")
		return ""
	} else {
		logging.PrintD(2, "Exiting with formatted date: %v", result)
		return result
	}
}

// yyyyMmDd converts inputted date strings into the user's defined format
func yyyyMmDd(fieldValue string) (string, bool) {

	logging.PrintD(3, "in yyyyMmDd function with string: '%s'", fieldValue)

	// Extract date part if it contains 'T'
	if strings.Contains(fieldValue, "T") {
		fieldValue = strings.Split(fieldValue, "T")[0]
	}

	// Remove existing hyphens
	fieldValue = strings.ReplaceAll(fieldValue, "-", "")

	if len(fieldValue) >= 8 {
		formatted := fieldValue[:4] + "-" + fieldValue[4:6] + "-" + fieldValue[6:8]
		logging.PrintD(2, "Formatted date: '%s'", formatted)
		return formatted, true // YYYY-MM-DD
	}

	// Return original value if no changes or date format invalid
	return fieldValue, false
}

// insertScrapedDate inserts the newly scraped date back into the JSON file
func insertScrapedMetadata(m *models.FileData, file *os.File, tag enums.WebClassTags) error {

	creds := m.MCredits
	dates := m.MDates
	titledesc := m.MTitleDesc

	logging.PrintD(3, "Entering insertScrapedData for file '%s'", m.JSONFilePath)

	if cmd.GetBool(keys.NoFileOverwrite) {
		err := backup.BackupFile(file)
		if err != nil {
			return fmt.Errorf("failed to create a backup of file '%s'", file.Name())
		}
	}

	// Seek to the beginning of the file
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to beginning of file: %w", err)
	}

	// Read the existing JSON content
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	logging.PrintD(3, "Got JSON content: %s", string(content))

	// Unmarshal the JSON into a map
	var jsonData map[string]interface{}
	err = json.Unmarshal(content, &jsonData)
	if err != nil {
		logging.PrintE(0, "Error unmarshalling JSON. Content: %s", string(content))
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	muMetaAdd.Lock()
	switch tag {
	// Dates and times
	case enums.WEBCLASS_DATE:

		if logging.Level >= 1 {
			printModel := shared.CreateModelPrintout(m, "Inserting date metadata into file '%s'", file.Name())
			logging.Print(printModel)
		}

		if reason, ok := updateJSONData(jsonData, consts.JCreationTime, dates.Creation_Time); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JCreationTime, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JDate, dates.Date); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JDate, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JReleaseDate, dates.ReleaseDate); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JReleaseDate, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JUploadDate, dates.UploadDate); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JUploadDate, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JYear, dates.Year); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JYear, reason)
		}

	// Descriptions
	case enums.WEBCLASS_DESCRIPTION:

		if logging.Level >= 1 {
			printModel := shared.CreateModelPrintout(m, "Inserting description metadata into file '%s'", file.Name())
			logging.Print(printModel)
		}

		if reason, ok := updateJSONData(jsonData, consts.JComment, titledesc.Comment); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JComment, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JDescription, titledesc.Description); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JDescription, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JLongDescription, titledesc.LongDescription); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JLongDescription, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JSummary, titledesc.Summary); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JSummary, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JSynopsis, titledesc.Synopsis); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JSynopsis, reason)
		}

	// Credits
	case enums.WEBCLASS_CREDITS:

		if logging.Level >= 1 {
			printModel := shared.CreateModelPrintout(m, "Inserting credits metadata into file '%s'", file.Name())
			logging.Print(printModel)
		}

		if reason, ok := updateJSONData(jsonData, consts.JActor, creds.Actor); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JActor, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JAuthor, creds.Author); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JAuthor, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JArtist, creds.Artist); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JArtist, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JComposer, creds.Composer); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JComposer, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JCreator, creds.Creator); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JCreator, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JDirector, creds.Director); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JDirector, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JPerformer, creds.Performer); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JPerformer, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JProducer, creds.Producer); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JProducer, reason)
		}

		if reason, ok := updateJSONData(jsonData, consts.JPublisher, creds.Publisher); !ok {
			logging.PrintD(2, "Skipped updating JSON file with field '%s' (%s)", consts.JPublisher, reason)
		}
	}

	// Marshal the updated JSON
	updatedContent, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		muMetaAdd.Unlock()
		return fmt.Errorf("failed to marshal updated JSON: %w", err)
	}

	// Truncate the file and write the updated content
	err = file.Truncate(0)
	if err != nil {
		muMetaAdd.Unlock()
		return fmt.Errorf("failed to truncate file: %w", err)
	}
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		muMetaAdd.Unlock()
		return fmt.Errorf("failed to seek to beginning of file: %w", err)
	}
	_, err = file.Write(updatedContent)
	if err != nil {
		muMetaAdd.Unlock()
		return fmt.Errorf("failed to write updated JSON to file: %w", err)
	}

	logging.PrintD(3, "Successfully updated JSON file with new date information")

	muMetaAdd.Unlock()
	return nil
}

// Unpack JSON decodes JSON for metafields
func unpackJSON(fieldType string, fieldMap map[string]*string, metadata map[string]interface{}) bool {

	var fillEmptyPriority []string
	dataFilled := false
	printMap := make(map[string]string)

	// Iterate through the decoded JSON to match fields against
	// the passed in map of fields to fill
	for key, value := range metadata {
		if strVal, ok := value.(string); ok {
			if field, exists := fieldMap[key]; exists && *field == "" {

				*field = strVal
				dataFilled = true

				fillEmptyPriority = append(fillEmptyPriority, *field)

				if printMap[key] == "" {
					printMap[key] = strVal
				}
			}
		}
	}

	// Iterate over the map of fields and attempt to fill the missing
	// fields (uses priority of the order of fields in the map)
	for _, value := range fieldMap {
		if *value == "" {
			for _, replacement := range fillEmptyPriority {
				if replacement != "" {
					*value = replacement
					dataFilled = true
					break
				}
			}
		}
	}

	printGrabbedFields(fieldType, &printMap)

	return dataFilled
}

// Print out the fetched fields
func printGrabbedFields(fieldType string, p *map[string]string) {

	printMap := *p

	muPrint.Lock()
	defer muPrint.Unlock()

	fmt.Println()
	logging.PrintI("Found and stored %s metadata fields from metafile:", fieldType)
	fmt.Println()

	for printKey, printVal := range printMap {
		if printKey != "" && printVal != "" {
			fmt.Printf(consts.ColorGreen + "Key: " + consts.ColorReset + printKey + consts.ColorYellow + "\nValue: " + consts.ColorReset + printVal + "\n")
		}
	}
	fmt.Println()
}

// updateJSONData is a helper function to update meta fields before writing back to the file
func updateJSONData(metadata map[string]interface{}, JTag string, value string) (string, bool) {

	if value != "" {
		if val, ok := metadata[JTag]; val == "" && ok {
			metadata[JTag] = val
			return "", true
		} else {
			return "field value already exists", false
		}
	} else {
		return fmt.Sprintf("model data field blank?:(%s)", value), false
	}
}
