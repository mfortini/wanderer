package migrations

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

func init() {
	m.Register(func(app core.App) error {
		jsonData := `{
			"createRule": "@request.auth.id != \"\" && author = @request.auth.id",
			"deleteRule": "@request.auth.id != \"\" && author = @request.auth.id",
			"fields": [
				{
					"autogeneratePattern": "[a-z0-9]{15}",
					"hidden": false,
					"id": "textassetid01",
					"max": 15,
					"min": 15,
					"name": "id",
					"pattern": "^[a-z0-9]+$",
					"presentable": false,
					"primaryKey": true,
					"required": true,
					"system": true,
					"type": "text"
				},
				{
					"hidden": false,
					"id": "selectassettyp",
					"maxSelect": 1,
					"name": "type",
					"presentable": false,
					"required": true,
					"system": false,
					"type": "select",
					"values": ["photo"]
				},
				{
					"hidden": false,
					"id": "fileassetfile1",
					"maxSelect": 1,
					"maxSize": 20971520,
					"mimeTypes": [
						"image/jpeg",
						"image/png",
						"image/vnd.mozilla.apng",
						"image/webp",
						"image/svg+xml",
						"image/heic",
						"video/mp4",
						"video/webm",
						"video/ogg"
					],
					"name": "file",
					"presentable": false,
					"protected": false,
					"required": false,
					"system": false,
					"thumbs": null,
					"type": "file"
				},
				{
					"hidden": false,
					"id": "selectassetsto",
					"maxSelect": 1,
					"name": "storage_mode",
					"presentable": false,
					"required": true,
					"system": false,
					"type": "select",
					"values": ["copy", "link_private", "link_public"]
				},
				{
					"hidden": false,
					"id": "selectassetrem",
					"maxSelect": 1,
					"name": "remote_status",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "select",
					"values": ["available", "missing", "inaccessible"]
				},
				{
					"cascadeDelete": true,
					"collectionId": "_pb_users_auth_",
					"hidden": false,
					"id": "relassetauth1",
					"maxSelect": 1,
					"minSelect": 0,
					"name": "author",
					"presentable": false,
					"required": true,
					"system": false,
					"type": "relation"
				},
				{
					"cascadeDelete": true,
					"collectionId": "e864strfxo14pm4",
					"hidden": false,
					"id": "relassettrail",
					"maxSelect": 1,
					"minSelect": 0,
					"name": "trail",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "relation"
				},
				{
					"cascadeDelete": true,
					"collectionId": "goeo2ubp103rzp9",
					"hidden": false,
					"id": "relassetwp001",
					"maxSelect": 1,
					"minSelect": 0,
					"name": "waypoint",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "relation"
				},
				{
					"cascadeDelete": true,
					"collectionId": "dd2l9a4vxpy2ni8",
					"hidden": false,
					"id": "relassetslog1",
					"maxSelect": 1,
					"minSelect": 0,
					"name": "summit_log",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "relation"
				},
				{
					"hidden": false,
					"id": "textassetprov",
					"max": 0,
					"min": 0,
					"name": "external_provider",
					"pattern": "",
					"presentable": false,
					"primaryKey": false,
					"required": false,
					"system": false,
					"type": "text"
				},
				{
					"hidden": false,
					"id": "textassetextid",
					"max": 0,
					"min": 0,
					"name": "external_id",
					"pattern": "",
					"presentable": false,
					"primaryKey": false,
					"required": false,
					"system": false,
					"type": "text"
				},
				{
					"hidden": false,
					"id": "dateassettaken",
					"max": "",
					"min": "",
					"name": "taken_at",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "date"
				},
				{
					"hidden": false,
					"id": "dateassetcheck",
					"max": "",
					"min": "",
					"name": "remote_checked_at",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "date"
				},
				{
					"hidden": false,
					"id": "dateassetmiss",
					"max": "",
					"min": "",
					"name": "remote_missing_since",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "date"
				},
				{
					"hidden": false,
					"id": "textasseterr1",
					"max": 0,
					"min": 0,
					"name": "remote_error",
					"pattern": "",
					"presentable": false,
					"primaryKey": false,
					"required": false,
					"system": false,
					"type": "text"
				},
				{
					"hidden": false,
					"id": "numassetlat01",
					"max": null,
					"min": null,
					"name": "lat",
					"onlyInt": false,
					"presentable": false,
					"required": false,
					"system": false,
					"type": "number"
				},
				{
					"hidden": false,
					"id": "numassetlon01",
					"max": null,
					"min": null,
					"name": "lon",
					"onlyInt": false,
					"presentable": false,
					"required": false,
					"system": false,
					"type": "number"
				},
				{
					"hidden": false,
					"id": "jsonassetmeta",
					"maxSize": 2000000,
					"name": "metadata",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "json"
				},
				{
					"hidden": false,
					"id": "autassetcreat",
					"name": "created",
					"onCreate": true,
					"onUpdate": false,
					"presentable": false,
					"system": false,
					"type": "autodate"
				},
				{
					"hidden": false,
					"id": "autassetupdat",
					"name": "updated",
					"onCreate": true,
					"onUpdate": true,
					"presentable": false,
					"system": false,
					"type": "autodate"
				}
			],
			"id": "assetcollect001",
			"indexes": [
				"CREATE INDEX ` + "`" + `idx_assets_external` + "`" + ` ON ` + "`" + `assets` + "`" + ` (` + "`" + `author` + "`" + `, ` + "`" + `external_provider` + "`" + `, ` + "`" + `external_id` + "`" + `)"
			],
			"listRule": "author = @request.auth.id || trail.author.user = @request.auth.id || trail.public = true || waypoint.trail.author.user = @request.auth.id || waypoint.trail.public = true || summit_log.author.user = @request.auth.id || summit_log.trail.public = true",
			"name": "assets",
			"system": false,
			"type": "base",
			"updateRule": "@request.auth.id != \"\" && author = @request.auth.id",
			"viewRule": "author = @request.auth.id || trail.author.user = @request.auth.id || trail.public = true || waypoint.trail.author.user = @request.auth.id || waypoint.trail.public = true || summit_log.author.user = @request.auth.id || summit_log.trail.public = true"
		}`

		collection := &core.Collection{}
		if err := json.Unmarshal([]byte(jsonData), &collection); err != nil {
			return err
		}
		if err := app.Save(collection); err != nil {
			return err
		}

		if err := migrateExistingPhotosToAssets(app, collection); err != nil {
			return err
		}
		if err := removeLegacyPhotoFields(app); err != nil {
			return err
		}

		integrations, err := app.FindCollectionByNameOrId("iz4sezoehde64wp")
		if err != nil {
			return err
		}
		if err := integrations.Fields.AddMarshaledJSONAt(5, []byte(`{
			"hidden": false,
			"id": "jsonimmich000",
			"maxSize": 2000000,
			"name": "immich",
			"presentable": false,
			"required": false,
			"system": false,
			"type": "json"
		}`)); err != nil {
			return err
		}

		return app.Save(integrations)
	}, func(app core.App) error {
		integrations, err := app.FindCollectionByNameOrId("iz4sezoehde64wp")
		if err == nil {
			integrations.Fields.RemoveById("jsonimmich000")
			if err := app.Save(integrations); err != nil {
				return err
			}
		}

		if err := restoreLegacyPhotoFields(app); err != nil {
			return err
		}

		collection, err := app.FindCollectionByNameOrId("assetcollect001")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}

type photoMigrationConfig struct {
	Collection string
	AssetField string
	LatField   string
	LonField   string
}

func migrateExistingPhotosToAssets(app core.App, assetCollection *core.Collection) error {
	configs := []photoMigrationConfig{
		{Collection: "trails", AssetField: "trail", LatField: "lat", LonField: "lon"},
		{Collection: "waypoints", AssetField: "waypoint", LatField: "lat", LonField: "lon"},
		{Collection: "summit_logs", AssetField: "summit_log"},
	}

	fsys, err := app.NewFilesystem()
	if err != nil {
		return err
	}
	defer fsys.Close()

	for _, cfg := range configs {
		records, err := app.FindAllRecords(cfg.Collection)
		if err != nil {
			return err
		}
		for _, record := range records {
			if err := migrateRecordPhotosToAssets(app, fsys, assetCollection, cfg, record); err != nil {
				return err
			}
		}
	}

	return nil
}

func migrateRecordPhotosToAssets(app core.App, fsys *filesystem.System, assetCollection *core.Collection, cfg photoMigrationConfig, source *core.Record) error {
	photos := source.GetStringSlice("photos")
	if len(photos) == 0 {
		return nil
	}

	author, err := resolveAssetAuthor(app, source.GetString("author"))
	if err != nil {
		return err
	}
	if author == "" {
		return fmt.Errorf("missing asset author for %s %s", cfg.Collection, source.Id)
	}

	for _, photo := range photos {
		reader, err := fsys.GetReader(source.BaseFilesPath() + "/" + photo)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return err
		}

		file, err := filesystem.NewFileFromBytes(data, photo)
		if err != nil {
			return err
		}

		asset := core.NewRecord(assetCollection)
		asset.Set("type", "photo")
		asset.Set("storage_mode", "copy")
		asset.Set("remote_status", "available")
		asset.Set("file", file)
		asset.Set("author", author)
		asset.Set(cfg.AssetField, source.Id)
		asset.Set("metadata", map[string]any{
			"source_collection": cfg.Collection,
			"source_record":     source.Id,
			"source_file":       photo,
		})
		if cfg.Collection == "waypoints" || cfg.Collection == "summit_logs" {
			if trail := source.GetString("trail"); trail != "" {
				asset.Set("trail", trail)
			}
		}
		if cfg.LatField != "" {
			asset.Set("lat", source.GetFloat(cfg.LatField))
		}
		if cfg.LonField != "" {
			asset.Set("lon", source.GetFloat(cfg.LonField))
		}

		if err := app.Save(asset); err != nil {
			return err
		}
	}

	source.Set("photos", []string{})
	return app.Save(source)
}

func removeLegacyPhotoFields(app core.App) error {
	fields := []struct {
		Collection string
		FieldID    string
	}{
		{Collection: "trails", FieldID: "aqbpyawe"},
		{Collection: "waypoints", FieldID: "tfhs3juh"},
		{Collection: "summit_logs", FieldID: "ixnksbkt"},
	}

	for _, f := range fields {
		collection, err := app.FindCollectionByNameOrId(f.Collection)
		if err != nil {
			return err
		}
		collection.Fields.RemoveById(f.FieldID)
		if err := app.Save(collection); err != nil {
			return err
		}
	}

	return nil
}

func restoreLegacyPhotoFields(app core.App) error {
	fields := []struct {
		Collection string
		FieldJSON  string
	}{
		{
			Collection: "trails",
			FieldJSON: `{
				"hidden": false,
				"id": "aqbpyawe",
				"maxSelect": 99,
				"maxSize": 20971520,
				"mimeTypes": ["image/jpeg","image/vnd.mozilla.apng","image/png","image/webp","image/svg+xml","image/heic","video/mp4","video/webm","video/ogg"],
				"name": "photos",
				"presentable": false,
				"protected": false,
				"required": false,
				"system": false,
				"thumbs": ["600x0"],
				"type": "file"
			}`,
		},
		{
			Collection: "waypoints",
			FieldJSON: `{
				"hidden": false,
				"id": "tfhs3juh",
				"maxSelect": 99,
				"maxSize": 20971520,
				"mimeTypes": ["image/jpeg","image/png","image/vnd.mozilla.apng","image/webp","image/svg+xml","video/ogg","video/mp4","video/webm"],
				"name": "photos",
				"presentable": false,
				"protected": false,
				"required": false,
				"system": false,
				"thumbs": null,
				"type": "file"
			}`,
		},
		{
			Collection: "summit_logs",
			FieldJSON: `{
				"hidden": false,
				"id": "ixnksbkt",
				"maxSelect": 99,
				"maxSize": 20971520,
				"mimeTypes": ["image/jpeg","image/png","image/vnd.mozilla.apng","image/webp","image/svg+xml","image/heic","video/ogg","video/mp4","video/webm"],
				"name": "photos",
				"presentable": false,
				"protected": false,
				"required": false,
				"system": false,
				"thumbs": null,
				"type": "file"
			}`,
		},
	}

	for _, f := range fields {
		collection, err := app.FindCollectionByNameOrId(f.Collection)
		if err != nil {
			return err
		}
		if err := collection.Fields.AddMarshaledJSON([]byte(f.FieldJSON)); err != nil {
			return err
		}
		if err := app.Save(collection); err != nil {
			return err
		}
	}

	return nil
}

func resolveAssetAuthor(app core.App, rawAuthor string) (string, error) {
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
