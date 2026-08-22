package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Trails included in a public list should be readable by anyone who can see
// that list. Without this, list expand already exposes trail metadata (and
// unprotected GPX/photos), but GET /api/v1/trail/{id} returns 403 — so
// clicking a trail on a public list does nothing.
const trailsPublicListClause = ` || (@collection.lists.trails.id ?= id && @collection.lists.public = true)`

const trailsBaseRule = `author.user = @request.auth.id || public = true || (@request.auth.id != "" && trail_share_via_trail.actor.user ?= @request.auth.id) || (trail_link_share_via_trail.token != "" && trail_link_share_via_trail.token = @request.query.share)`

const waypointsBaseRule = `author = @request.auth.id || trail.author.user ?= @request.auth.id || trail.public ?= true || trail.trail_share_via_trail.actor.user ?= @request.auth.id || (trail.trail_link_share_via_trail.token != "" && trail.trail_link_share_via_trail.token = @request.query.share)`

const waypointsPublicListClause = ` || (@collection.lists.trails.id ?= trail && @collection.lists.public = true)`

const summitLogsBaseRule = `(@request.auth.id != "" && (author.user = @request.auth.id || trail.author.user = @request.auth.id)) || trail.public = true || trail.trail_share_via_trail.actor.user ?= @request.auth.id`

const summitLogsPublicListClause = ` || (@collection.lists.trails.id ?= trail && @collection.lists.public = true)`

func init() {
	m.Register(func(app core.App) error {
		if err := setCollectionRules(app, "e864strfxo14pm4", trailsBaseRule+trailsPublicListClause); err != nil {
			return err
		}
		if err := setCollectionRules(app, "waypoints", waypointsBaseRule+waypointsPublicListClause); err != nil {
			return err
		}
		return setCollectionRules(app, "summit_logs", summitLogsBaseRule+summitLogsPublicListClause)
	}, func(app core.App) error {
		if err := setCollectionRules(app, "e864strfxo14pm4", trailsBaseRule); err != nil {
			return err
		}
		if err := setCollectionRules(app, "waypoints", waypointsBaseRule); err != nil {
			return err
		}
		return setCollectionRules(app, "summit_logs", summitLogsBaseRule)
	})
}

func setCollectionRules(app core.App, nameOrId, rule string) error {
	collection, err := app.FindCollectionByNameOrId(nameOrId)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{
		"listRule": rule,
		"viewRule": rule,
	})
	if err != nil {
		return err
	}
	if err := json.Unmarshal(payload, &collection); err != nil {
		return err
	}
	return app.Save(collection)
}
