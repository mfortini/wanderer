package util

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

type PhotoAssetInput struct {
	Author    string
	Trail     string
	Waypoint  string
	SummitLog string
	Lat       float64
	Lon       float64
	File      *filesystem.File
	Metadata  map[string]any
}

func CreatePhotoAsset(app core.App, input PhotoAssetInput) error {
	if input.File == nil {
		return nil
	}

	author, err := ResolveAssetAuthor(app, input.Author)
	if err != nil {
		return err
	}
	if author == "" {
		return fmt.Errorf("missing asset author")
	}

	collection, err := app.FindCollectionByNameOrId("assets")
	if err != nil {
		return err
	}

	record := core.NewRecord(collection)
	record.Set("type", "photo")
	record.Set("storage_mode", "copy")
	record.Set("remote_status", "available")
	record.Set("author", author)
	record.Set("file", input.File)
	if input.Trail != "" {
		record.Set("trail", input.Trail)
	}
	if input.Waypoint != "" {
		record.Set("waypoint", input.Waypoint)
	}
	if input.SummitLog != "" {
		record.Set("summit_log", input.SummitLog)
	}
	if input.Lat != 0 {
		record.Set("lat", input.Lat)
	}
	if input.Lon != 0 {
		record.Set("lon", input.Lon)
	}
	if input.Metadata != nil {
		record.Set("metadata", input.Metadata)
	}

	return app.Save(record)
}

func ResolveAssetAuthor(app core.App, rawAuthor string) (string, error) {
	if rawAuthor == "" {
		return "", nil
	}
	if _, err := app.FindRecordById("_pb_users_auth_", rawAuthor); err == nil {
		return rawAuthor, nil
	}
	actor, err := app.FindRecordById("activitypub_actors", rawAuthor)
	if err != nil {
		return rawAuthor, nil
	}
	return actor.GetString("user"), nil
}

func PhotoAssetURLs(app core.App, targetField string, targetID string, origin string, limit int) ([]string, error) {
	switch targetField {
	case "trail", "waypoint", "summit_log":
	default:
		return nil, fmt.Errorf("unsupported asset relation %q", targetField)
	}
	if targetID == "" {
		return nil, nil
	}

	records, err := app.FindRecordsByFilter("assets", targetField+"={:id} && type='photo'", "created", limit, 0, dbx.Params{"id": targetID})
	if err != nil {
		return nil, err
	}

	urls := make([]string, 0, len(records))
	for _, record := range records {
		if file := record.GetString("file"); file != "" {
			urls = append(urls, fmt.Sprintf("%s/api/v1/files/%s/%s/%s", origin, record.Collection().Id, record.Id, file))
			continue
		}
		if record.GetString("storage_mode") == "" || record.GetString("storage_mode") == "copy" {
			continue
		}
		urls = append(urls, fmt.Sprintf("%s/api/v1/assets/%s/file", origin, record.Id))
	}
	return urls, nil
}
