package immich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"pocketbase/util"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/tkrajina/gpxgo/gpx"
)

type Integration struct {
	Active            bool     `json:"active"`
	URL               string   `json:"url"`
	ApiKey            string   `json:"apiKey"`
	TimeWindowMinutes int      `json:"timeWindowMinutes"`
	MaxDistanceMeters int      `json:"maxDistanceMeters"`
	MaxWaypoints      int      `json:"maxWaypoints"`
	PhotoMode         string   `json:"photoMode"`
	Providers         []string `json:"providers"`
	UseForStrava      bool     `json:"useForStrava"`
	UseForKomoot      bool     `json:"useForKomoot"`
	UseForHammerhead  bool     `json:"useForHammerhead"`
	UseForUpload      bool     `json:"useForUpload"`
}

var immichAssetIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func IsAssetID(assetID string) bool {
	return immichAssetIDPattern.MatchString(assetID)
}

type metadataSearchResponse struct {
	Assets struct {
		Items    []Asset `json:"items"`
		NextPage *string `json:"nextPage"`
	} `json:"assets"`
}

type Asset struct {
	ID               string   `json:"id"`
	FileCreatedAt    string   `json:"fileCreatedAt"`
	OriginalFileName string   `json:"originalFileName"`
	ExifInfo         ExifInfo `json:"exifInfo"`
}

type ExifInfo struct {
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	City        string   `json:"city"`
	Country     string   `json:"country"`
	Description string   `json:"description"`
}

type Candidate struct {
	AssetID           string  `json:"assetId"`
	OriginalFileName  string  `json:"originalFileName"`
	TakenAt           string  `json:"takenAt"`
	Lat               float64 `json:"lat"`
	Lon               float64 `json:"lon"`
	Distance          float64 `json:"distance"`
	PointLat          float64 `json:"pointLat"`
	PointLon          float64 `json:"pointLon"`
	DistanceFromStart float64 `json:"distanceFromStart"`
	City              string  `json:"city"`
	Country           string  `json:"country"`
}

type CandidatesResult struct {
	HasTimestamps bool        `json:"hasTimestamps"`
	Candidates    []Candidate `json:"candidates"`
	HasMore       bool        `json:"hasMore"`
	TakenAfter    string      `json:"takenAfter"`
}

type trackPoint struct {
	Lat       float64
	Lon       float64
	Distance  float64
	Timestamp *time.Time
}

type assetMatch struct {
	asset    Asset
	point    trackPoint
	distance float64
}

var httpClient = &http.Client{
	Timeout:       30 * time.Second,
	Transport:     newSafeTransport(),
	CheckRedirect: validateRedirect,
}

const duplicateCoordinateDistanceMeters = 5.0
const (
	PhotoModeCopy        = "copy"
	PhotoModeLinkPrivate = "link_private"
	PhotoModeLinkPublic  = "link_public"
)

func ParseIntegration(raw string, encryptionKey string) (*Integration, error) {
	return parseIntegration(raw, encryptionKey, true)
}

func ParseStoredIntegration(raw string, encryptionKey string) (*Integration, error) {
	return parseIntegration(raw, encryptionKey, false)
}

func parseIntegration(raw string, encryptionKey string, requireActive bool) (*Integration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var cfg Integration
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	if requireActive && !cfg.Active {
		return nil, nil
	}
	cfg.normalize()
	if cfg.ApiKey != "" && encryptionKey != "" && util.CanDecryptSecret(cfg.ApiKey) {
		decrypted, err := security.Decrypt(cfg.ApiKey, encryptionKey)
		if err != nil {
			return nil, err
		}
		cfg.ApiKey = string(decrypted)
	}
	return &cfg, nil
}

func ParseIntegrationFromRecord(record *core.Record) (*Integration, error) {
	return ParseIntegration(record.GetString("immich"), os.Getenv("POCKETBASE_ENCRYPTION_KEY"))
}

func ParseStoredIntegrationFromRecord(record *core.Record) (*Integration, error) {
	return ParseStoredIntegration(record.GetString("immich"), os.Getenv("POCKETBASE_ENCRYPTION_KEY"))
}

func (cfg *Integration) normalize() {
	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.TimeWindowMinutes <= 0 {
		cfg.TimeWindowMinutes = 120
	}
	if cfg.MaxDistanceMeters <= 0 {
		cfg.MaxDistanceMeters = 150
	}
	if cfg.MaxWaypoints <= 0 {
		cfg.MaxWaypoints = 25
	}
	cfg.PhotoMode = NormalizePhotoMode(cfg.PhotoMode)
	if cfg.PhotoMode == "" {
		cfg.PhotoMode = PhotoModeCopy
	}
	if len(cfg.Providers) == 0 {
		if cfg.UseForStrava {
			cfg.Providers = append(cfg.Providers, "strava")
		}
		if cfg.UseForKomoot {
			cfg.Providers = append(cfg.Providers, "komoot")
		}
		if cfg.UseForHammerhead {
			cfg.Providers = append(cfg.Providers, "hammerhead")
		}
		if cfg.UseForUpload {
			cfg.Providers = append(cfg.Providers, "upload")
		}
	}
}

func NormalizePhotoMode(mode string) string {
	switch mode {
	case PhotoModeCopy:
		return PhotoModeCopy
	case PhotoModeLinkPrivate:
		return PhotoModeLinkPrivate
	case PhotoModeLinkPublic:
		return PhotoModeLinkPublic
	default:
		return ""
	}
}

func IsRemotePhotoMode(mode string) bool {
	normalized := NormalizePhotoMode(mode)
	return normalized != "" && normalized != PhotoModeCopy
}

func IsPrivateLinkPhotoMode(mode string) bool {
	return NormalizePhotoMode(mode) == PhotoModeLinkPrivate
}

func (cfg *Integration) ShouldUseFor(provider string) bool {
	if cfg == nil || !cfg.Active || cfg.ApiKey == "" || cfg.URL == "" {
		return false
	}
	for _, enabled := range cfg.Providers {
		if enabled == provider {
			return true
		}
	}
	return false
}

func CheckConnection(cfg *Integration) error {
	if cfg == nil {
		return fmt.Errorf("immich configuration is required")
	}
	cfg.normalize()
	if cfg.URL == "" {
		return fmt.Errorf("immich url is required")
	}
	if cfg.ApiKey == "" {
		return fmt.Errorf("immich api key is required")
	}
	baseURL, err := cfg.safeBaseURL()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = fetchAssets(baseURL, cfg.ApiKey, now.Add(-time.Hour), now, 1)
	if err != nil {
		return err
	}
	return nil
}

func (cfg *Integration) baseURL() string {
	base := strings.TrimSpace(cfg.URL)
	if base == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(base), "http://") && !strings.HasPrefix(strings.ToLower(base), "https://") {
		base = "https://" + base
	}
	base = strings.TrimRight(base, "/")
	if !strings.HasSuffix(base, "/api") {
		base += "/api"
	}
	return base
}

func (cfg *Integration) safeBaseURL() (string, error) {
	base := cfg.baseURL()
	if base == "" {
		return "", nil
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid immich url: %w", err)
	}
	if err := validateOutboundURL(parsed); err != nil {
		return "", err
	}
	return base, nil
}

func newSafeTransport() http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = safeDialContext
	return transport
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, addr := range ips {
		if isBlockedOutboundIP(addr.IP) {
			return nil, fmt.Errorf("immich url resolves to a private or local address")
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("immich url did not resolve to an address")
	}
	for _, addr := range ips {
		if ipMatchesNetwork(addr.IP, network) {
			dialer := &net.Dialer{Timeout: 30 * time.Second}
			return dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
		}
	}
	return nil, fmt.Errorf("immich url did not resolve to a compatible address")
}

func validateRedirect(req *http.Request, via []*http.Request) error {
	return validateOutboundURL(req.URL)
}

func validateOutboundURL(parsed *url.URL) error {
	if parsed == nil {
		return fmt.Errorf("immich url is required")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("immich url must use http or https")
	}
	if parsed.User != nil {
		return fmt.Errorf("immich url must not include credentials")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("immich url host is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolving immich url: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("immich url did not resolve to an address")
	}
	for _, addr := range ips {
		if isBlockedOutboundIP(addr.IP) {
			return fmt.Errorf("immich url resolves to a private or local address")
		}
	}
	return nil
}

func isBlockedOutboundIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

func ipMatchesNetwork(ip net.IP, network string) bool {
	if strings.HasSuffix(network, "4") {
		return ip.To4() != nil
	}
	if strings.HasSuffix(network, "6") {
		return ip.To4() == nil
	}
	return true
}

func AttachWaypointsFromGPX(app core.App, cfg *Integration, userID, trailID string, gpxFile *filesystem.File) error {
	if cfg == nil || cfg.ApiKey == "" || gpxFile == nil {
		return nil
	}
	points, err := loadTrackPoints(gpxFile)
	if err != nil {
		return err
	}
	return attachWaypointsFromPoints(app, cfg, userID, trailID, points)
}

func AttachTrailForProvider(app core.App, userID, trailID, provider string) error {
	app.Logger().Info("immich attach loading integration", "user", userID, "trail", trailID, "provider", provider)
	integrations, err := app.FindRecordsByFilter("integrations", "user={:user}", "", 1, 0, dbx.Params{"user": userID})
	if err != nil {
		return err
	}
	if len(integrations) == 0 {
		app.Logger().Info("immich attach skipped: no integration record", "user", userID, "trail", trailID)
		return nil
	}

	cfg, err := ParseIntegrationFromRecord(integrations[0])
	if err != nil {
		return err
	}
	if cfg == nil || !cfg.ShouldUseFor(provider) {
		app.Logger().Info("immich attach skipped: integration inactive or provider disabled", "user", userID, "trail", trailID, "provider", provider)
		return nil
	}

	trail, err := app.FindRecordById("trails", trailID)
	if err != nil {
		return err
	}
	gpxName := trail.GetString("gpx")
	if gpxName == "" {
		app.Logger().Info("immich attach skipped: trail has no gpx", "user", userID, "trail", trailID)
		return nil
	}

	fsys, err := app.NewFilesystem()
	if err != nil {
		return err
	}
	defer fsys.Close()

	reader, err := fsys.GetReader(trail.BaseFilesPath() + "/" + gpxName)
	if err != nil {
		return err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	points, err := loadTrackPointsFromBytes(data)
	if err != nil {
		return err
	}
	app.Logger().Info("immich attach loaded gpx", "user", userID, "trail", trailID, "points", len(points), "pointsWithTimestamp", countTrackPointsWithTimestamp(points), "gpx", gpxName)

	return attachWaypointsFromPoints(app, cfg, userID, trailID, points)
}

func attachWaypointsFromPoints(app core.App, cfg *Integration, userID, trailID string, points []trackPoint) error {
	if cfg == nil || cfg.ApiKey == "" {
		app.Logger().Info("immich attach skipped: missing config or api key", "user", userID, "trail", trailID)
		return nil
	}
	baseURL, err := cfg.safeBaseURL()
	if err != nil {
		return err
	}
	if baseURL == "" || len(points) < 2 {
		app.Logger().Info("immich attach skipped: missing base url or too few points", "user", userID, "trail", trailID, "points", len(points))
		return nil
	}
	boundsStart, boundsEnd, ok := timeBounds(points)
	if !ok {
		app.Logger().Info("immich attach skipped: gpx has no timestamps", "user", userID, "trail", trailID, "points", len(points))
		return nil
	}
	window := time.Duration(cfg.TimeWindowMinutes) * time.Minute
	app.Logger().Info("immich attach searching assets", "user", userID, "trail", trailID, "from", boundsStart.Add(-window), "to", boundsEnd.Add(window), "maxDistance", cfg.MaxDistanceMeters, "photoMode", cfg.PhotoMode)
	assets, err := fetchAssets(baseURL, cfg.ApiKey, boundsStart.Add(-window), boundsEnd.Add(window), cfg.MaxWaypoints*3)
	if err != nil {
		return err
	}
	app.Logger().Info("immich attach fetched assets", "user", userID, "trail", trailID, "assets", len(assets))
	matches := matchAssets(points, assets, cfg.MaxDistanceMeters)
	if len(matches) == 0 {
		app.Logger().Info("immich attach skipped: no matching geotagged assets near route", "user", userID, "trail", trailID, "assets", len(assets), "maxDistance", cfg.MaxDistanceMeters)
		return nil
	}
	app.Logger().Info("immich attach matched assets", "user", userID, "trail", trailID, "matches", len(matches))

	waypointCollection, err := app.FindCollectionByNameOrId("waypoints")
	if err != nil {
		return err
	}
	waypoints, err := app.FindRecordsByFilter("waypoints", "trail={:trail}", "", -1, 0, dbx.Params{"trail": trailID})
	if err != nil {
		return err
	}

	created := 0
	for _, match := range matches {
		if exists, err := importedAssetExists(app, userID, match.asset.ID); err != nil {
			return err
		} else if exists {
			app.Logger().Info("immich attach skipped existing asset", "user", userID, "trail", trailID, "asset", match.asset.ID)
			continue
		}

		target := findWaypointByCoordinate(waypoints, match.point.Lat, match.point.Lon)
		if target == nil {
			if created >= cfg.MaxWaypoints {
				continue
			}
			target = core.NewRecord(waypointCollection)
			target.Load(map[string]any{
				"name":                buildWaypointName(match.asset),
				"lat":                 match.point.Lat,
				"lon":                 match.point.Lon,
				"icon":                "camera",
				"author":              userID,
				"distance_from_start": match.point.Distance,
				"trail":               trailID,
			})
			if desc := strings.TrimSpace(match.asset.ExifInfo.Description); desc != "" {
				target.Set("description", desc)
			}
			if err := app.Save(target); err != nil {
				return err
			}
			waypoints = append(waypoints, target)
			created++
			app.Logger().Info("immich attach created waypoint", "user", userID, "trail", trailID, "waypoint", target.Id, "asset", match.asset.ID, "distance", match.distance)
		}

		photoMode := effectivePhotoModeForTrail(app, cfg.PhotoMode, trailID)
		var photo *filesystem.File
		if photoMode == PhotoModeCopy {
			var err error
			photo, err = downloadAsset(baseURL, cfg.ApiKey, match.asset)
			if err != nil {
				return err
			}
			if photo == nil {
				continue
			}
		}
		if err := createPhotoAsset(app, userID, trailID, target.Id, match, photoMode, photo); err != nil {
			return err
		}
		app.Logger().Info("immich attach created asset", "user", userID, "trail", trailID, "waypoint", target.Id, "asset", match.asset.ID, "photoMode", photoMode)
	}
	app.Logger().Info("immich attach completed matching loop", "user", userID, "trail", trailID)
	return nil
}

func importedAssetExists(app core.App, userID, externalID string) (bool, error) {
	if externalID == "" {
		return false, nil
	}
	records, err := app.FindRecordsByFilter(
		"assets",
		"author={:author} && external_provider={:provider} && external_id={:external_id}",
		"",
		1,
		0,
		dbx.Params{"author": userID, "provider": "immich", "external_id": externalID},
	)
	if err != nil {
		return false, err
	}
	return len(records) > 0, nil
}

func createPhotoAsset(app core.App, userID, trailID, waypointID string, match assetMatch, photoMode string, photo *filesystem.File) error {
	collection, err := app.FindCollectionByNameOrId("assets")
	if err != nil {
		return err
	}
	photoMode = NormalizePhotoMode(photoMode)
	if photoMode == "" {
		photoMode = PhotoModeCopy
	}

	record := core.NewRecord(collection)
	record.Load(map[string]any{
		"type":              "photo",
		"storage_mode":      photoMode,
		"remote_status":     "available",
		"author":            userID,
		"trail":             trailID,
		"waypoint":          waypointID,
		"external_provider": "immich",
		"external_id":       match.asset.ID,
		"lat":               match.point.Lat,
		"lon":               match.point.Lon,
		"metadata": map[string]any{
			"originalFileName": match.asset.OriginalFileName,
			"distanceMeters":   match.distance,
			"exif":             match.asset.ExifInfo,
		},
	})
	if match.asset.FileCreatedAt != "" {
		record.Set("taken_at", match.asset.FileCreatedAt)
	}
	if photo != nil {
		record.Set("file", photo)
	}

	return app.Save(record)
}

func effectivePhotoModeForTrail(app core.App, photoMode, trailID string) string {
	photoMode = NormalizePhotoMode(photoMode)
	if photoMode == "" {
		photoMode = PhotoModeCopy
	}
	if photoMode == PhotoModeLinkPrivate {
		trail, err := app.FindRecordById("trails", trailID)
		if err == nil && trail.GetBool("public") {
			return PhotoModeCopy
		}
	}
	return photoMode
}

func OpenRemoteAsset(app core.App, asset *core.Record) (*http.Response, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is required")
	}
	if asset.GetString("external_provider") != "immich" || asset.GetString("external_id") == "" {
		return nil, fmt.Errorf("asset is not an immich asset")
	}

	integrations, err := app.FindRecordsByFilter("integrations", "user={:user}", "", 1, 0, dbx.Params{"user": asset.GetString("author")})
	if err != nil {
		return nil, err
	}
	if len(integrations) == 0 {
		return nil, fmt.Errorf("immich integration not found")
	}

	cfg, err := ParseIntegrationFromRecord(integrations[0])
	if err != nil {
		return nil, err
	}
	if cfg == nil || cfg.ApiKey == "" {
		return nil, fmt.Errorf("immich integration is not configured")
	}
	baseURL, err := cfg.safeBaseURL()
	if err != nil {
		return nil, err
	}
	if baseURL == "" {
		return nil, fmt.Errorf("immich integration is not configured")
	}

	return openRemoteAsset(baseURL, cfg.ApiKey, asset.GetString("external_id"))
}

// MaterializeRemoteAssetsForUser downloads all remote Immich assets across all
// of the user's trails. Called explicitly when the user chooses to download
// existing linked photos after changing the photo mode.
func MaterializeRemoteAssetsForUser(app core.App, userID string) error {
	actors, err := app.FindRecordsByFilter(
		"activitypub_actors", "user={:user}", "", -1, 0, dbx.Params{"user": userID},
	)
	if err != nil || len(actors) == 0 {
		return nil
	}

	for _, actor := range actors {
		trails, err := app.FindRecordsByFilter(
			"trails", "author={:author}", "", -1, 0,
			dbx.Params{"author": actor.Id},
		)
		if err != nil {
			return err
		}
		for _, trail := range trails {
			if err := materializeAllRemoteAssetsForTrail(app, trail.Id); err != nil {
				return err
			}
		}
	}
	return nil
}

// MaterializeRemoteAssetsForUserPublicTrails downloads remote assets only on
// public trails. Used when the trail-publish hook needs to catch assets that
// were not yet materialized.
func MaterializeRemoteAssetsForUserPublicTrails(app core.App, userID string) error {
	actors, err := app.FindRecordsByFilter(
		"activitypub_actors", "user={:user}", "", -1, 0, dbx.Params{"user": userID},
	)
	if err != nil || len(actors) == 0 {
		return nil
	}

	for _, actor := range actors {
		trails, err := app.FindRecordsByFilter(
			"trails", "author={:author} && public=true", "", -1, 0,
			dbx.Params{"author": actor.Id},
		)
		if err != nil {
			return err
		}
		for _, trail := range trails {
			if err := materializeAllRemoteAssetsForTrail(app, trail.Id); err != nil {
				return err
			}
		}
	}
	return nil
}

func materializeAllRemoteAssetsForTrail(app core.App, trailID string) error {
	assets, err := app.FindRecordsByFilter(
		"assets",
		"trail={:trail} && external_provider='immich' && external_id!=''",
		"", -1, 0,
		dbx.Params{"trail": trailID},
	)
	if err != nil {
		return err
	}
	for _, asset := range assets {
		if !IsRemotePhotoMode(asset.GetString("storage_mode")) {
			continue
		}
		if err := MaterializeRemoteAsset(app, asset); err != nil {
			return fmt.Errorf("materializing immich asset %s: %w", asset.Id, err)
		}
	}
	return nil
}

func MaterializePrivateLinkedAssetsForTrail(app core.App, trailID string) error {
	if strings.TrimSpace(trailID) == "" {
		return nil
	}
	assets, err := app.FindRecordsByFilter(
		"assets",
		"trail={:trail} && external_provider='immich' && external_id!='' && storage_mode={:link_private}",
		"",
		-1,
		0,
		dbx.Params{
			"trail":        trailID,
			"link_private": PhotoModeLinkPrivate,
		},
	)
	if err != nil {
		return err
	}
	for _, asset := range assets {
		if err := MaterializeRemoteAsset(app, asset); err != nil {
			return fmt.Errorf("materializing immich asset %s: %w", asset.Id, err)
		}
	}
	return nil
}

func MaterializeRemoteAsset(app core.App, asset *core.Record) error {
	if asset == nil {
		return nil
	}
	if asset.GetString("external_provider") != "immich" || asset.GetString("external_id") == "" {
		return fmt.Errorf("asset is not an immich asset")
	}

	// Re-read from DB so a concurrent request that already finished materializing
	// short-circuits here instead of downloading the file a second time.
	fresh, err := app.FindRecordById("assets", asset.Id)
	if err != nil {
		return err
	}
	if !IsRemotePhotoMode(fresh.GetString("storage_mode")) {
		asset.Set("file", fresh.GetString("file"))
		asset.Set("storage_mode", fresh.GetString("storage_mode"))
		return nil
	}

	integrations, err := app.FindRecordsByFilter("integrations", "user={:user}", "", 1, 0, dbx.Params{"user": asset.GetString("author")})
	if err != nil {
		return err
	}
	if len(integrations) == 0 {
		return fmt.Errorf("immich integration not found")
	}
	cfg, err := ParseStoredIntegrationFromRecord(integrations[0])
	if err != nil {
		return err
	}
	if cfg == nil || cfg.ApiKey == "" {
		return fmt.Errorf("immich integration is not configured")
	}
	baseURL, err := cfg.safeBaseURL()
	if err != nil {
		return err
	}
	if baseURL == "" {
		return fmt.Errorf("immich integration is not configured")
	}

	remoteAsset := Asset{ID: asset.GetString("external_id")}
	if fetched, err := fetchAssetByID(baseURL, cfg.ApiKey, remoteAsset.ID); err == nil && fetched != nil {
		remoteAsset = *fetched
	}
	photo, err := downloadAsset(baseURL, cfg.ApiKey, remoteAsset)
	if err != nil {
		return err
	}
	if photo == nil {
		return fmt.Errorf("immich asset could not be downloaded")
	}
	asset.Set("file", photo)
	asset.Set("storage_mode", PhotoModeCopy)
	asset.Set("remote_status", "available")
	asset.Set("remote_missing_since", nil)
	asset.Set("remote_error", "")
	asset.Set("remote_checked_at", time.Now().UTC().Format(time.RFC3339))
	return app.Save(asset)
}

func MarkRemoteStatus(app core.App, asset *core.Record, status string, err error) error {
	if asset == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	asset.Set("remote_status", status)
	asset.Set("remote_checked_at", now)
	if status == "missing" && asset.GetString("remote_missing_since") == "" {
		asset.Set("remote_missing_since", now)
	}
	if status == "available" {
		asset.Set("remote_missing_since", nil)
		asset.Set("remote_error", "")
	} else if err != nil {
		asset.Set("remote_error", err.Error())
	}
	return app.Save(asset)
}

func loadTrackPoints(gpxFile *filesystem.File) ([]trackPoint, error) {
	reader, err := gpxFile.Reader.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	buf, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return loadTrackPointsFromBytes(buf)
}

func loadTrackPointsFromBytes(buf []byte) ([]trackPoint, error) {
	doc, err := gpx.ParseBytes(buf)
	if err != nil {
		return nil, err
	}

	var points []trackPoint
	var distance float64
	appendPoint := func(lat, lon float64, timestamp time.Time) {
		if len(points) > 0 {
			prev := points[len(points)-1]
			distance += haversineDistance(prev.Lat, prev.Lon, lat, lon)
		}
		var tsPtr *time.Time
		if !timestamp.IsZero() {
			ts := timestamp
			tsPtr = &ts
		}
		points = append(points, trackPoint{Lat: lat, Lon: lon, Distance: distance, Timestamp: tsPtr})
	}

	for _, track := range doc.Tracks {
		for _, segment := range track.Segments {
			for _, pt := range segment.Points {
				appendPoint(pt.Point.Latitude, pt.Point.Longitude, pt.Timestamp)
			}
		}
	}
	if len(points) == 0 {
		for _, route := range doc.Routes {
			for _, pt := range route.Points {
				appendPoint(pt.Point.Latitude, pt.Point.Longitude, pt.Timestamp)
			}
		}
	}
	return points, nil
}

func timeBounds(points []trackPoint) (time.Time, time.Time, bool) {
	var start, end time.Time
	for _, p := range points {
		if p.Timestamp == nil || p.Timestamp.IsZero() {
			continue
		}
		if start.IsZero() || p.Timestamp.Before(start) {
			start = *p.Timestamp
		}
		if end.IsZero() || p.Timestamp.After(end) {
			end = *p.Timestamp
		}
	}
	if start.IsZero() || end.IsZero() {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func countTrackPointsWithTimestamp(points []trackPoint) int {
	count := 0
	for _, point := range points {
		if point.Timestamp != nil && !point.Timestamp.IsZero() {
			count++
		}
	}
	return count
}

func fetchAssets(baseURL, apiKey string, takenAfter, takenBefore time.Time, maxAssets int) ([]Asset, error) {
	if maxAssets <= 0 {
		maxAssets = 75
	}
	assets := make([]Asset, 0, maxAssets)
	page := 1
	for len(assets) < maxAssets {
		payload, _ := json.Marshal(map[string]any{
			"withExif":    true,
			"type":        "IMAGE",
			"takenAfter":  takenAfter.Format(time.RFC3339),
			"takenBefore": takenBefore.Format(time.RFC3339),
			"page":        page,
		})
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/search/metadata", baseURL), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", apiKey)
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("immich search failed: %d", resp.StatusCode)
		}
		var parsed metadataSearchResponse
		if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
			return nil, err
		}
		assets = append(assets, parsed.Assets.Items...)
		if parsed.Assets.NextPage == nil || *parsed.Assets.NextPage == "" {
			break
		}
		nextPage, ok := parseNextPage(*parsed.Assets.NextPage)
		if !ok {
			break
		}
		page = nextPage
	}
	if len(assets) > maxAssets {
		assets = assets[:maxAssets]
	}
	return assets, nil
}

func parseNextPage(raw string) (int, bool) {
	if n, err := strconv.Atoi(raw); err == nil {
		return n, true
	}
	if idx := strings.Index(raw, "page="); idx >= 0 {
		part := raw[idx+5:]
		for i, r := range part {
			if r < '0' || r > '9' {
				part = part[:i]
				break
			}
		}
		if n, err := strconv.Atoi(part); err == nil {
			return n, true
		}
	}
	return 0, false
}

func matchAssets(points []trackPoint, assets []Asset, maxDistance int) []assetMatch {
	threshold := float64(maxDistance)
	if threshold <= 0 {
		threshold = 150
	}
	matches := make([]assetMatch, 0, len(assets))
	for _, asset := range assets {
		if asset.ExifInfo.Latitude == nil || asset.ExifInfo.Longitude == nil {
			continue
		}
		point, distance := findNearestTrackPoint(*asset.ExifInfo.Latitude, *asset.ExifInfo.Longitude, points)
		if point == nil || distance > threshold {
			continue
		}
		matches = append(matches, assetMatch{asset: asset, point: *point, distance: distance})
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].point.Distance < matches[j].point.Distance
	})
	return matches
}

func findNearestTrackPoint(lat, lon float64, points []trackPoint) (*trackPoint, float64) {
	var nearest *trackPoint
	var nearestDistance float64
	for i := range points {
		distance := haversineDistance(lat, lon, points[i].Lat, points[i].Lon)
		if nearest == nil || distance < nearestDistance {
			nearest = &points[i]
			nearestDistance = distance
		}
	}
	return nearest, nearestDistance
}

func findWaypointByCoordinate(records []*core.Record, lat, lon float64) *core.Record {
	for _, record := range records {
		if haversineDistance(lat, lon, record.GetFloat("lat"), record.GetFloat("lon")) <= duplicateCoordinateDistanceMeters {
			return record
		}
	}
	return nil
}

func downloadAsset(baseURL, apiKey string, asset Asset) (*filesystem.File, error) {
	if !IsAssetID(asset.ID) {
		return nil, fmt.Errorf("invalid immich asset id")
	}
	assetID := url.PathEscape(asset.ID)
	attempts := []string{
		fmt.Sprintf("%s/assets/%s/original", baseURL, assetID),
		fmt.Sprintf("%s/assets/%s/thumbnail?size=preview", baseURL, assetID),
	}
	for _, url := range attempts {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", apiKey)
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			continue
		}
		name := asset.ID + ".jpg"
		if strings.TrimSpace(asset.OriginalFileName) != "" {
			name = asset.OriginalFileName
		}
		return filesystem.NewFileFromBytes(data, name)
	}
	return nil, nil
}

func openRemoteAsset(baseURL, apiKey, assetID string) (*http.Response, error) {
	if !IsAssetID(assetID) {
		return nil, fmt.Errorf("invalid immich asset id")
	}
	assetID = url.PathEscape(assetID)
	attempts := []string{
		fmt.Sprintf("%s/assets/%s/original", baseURL, assetID),
		fmt.Sprintf("%s/assets/%s/thumbnail?size=preview", baseURL, assetID),
	}
	var lastErr error
	for _, url := range attempts {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", apiKey)
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		lastErr = fmt.Errorf("immich asset fetch failed: %d", resp.StatusCode)
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return resp, lastErr
		}
		resp.Body.Close()
	}
	return nil, lastErr
}

func buildWaypointName(asset Asset) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(asset.ExifInfo.City) != "" {
		parts = append(parts, strings.TrimSpace(asset.ExifInfo.City))
	}
	if strings.TrimSpace(asset.ExifInfo.Country) != "" {
		parts = append(parts, strings.TrimSpace(asset.ExifInfo.Country))
	}
	if len(parts) == 0 {
		return "Imported from Immich"
	}
	return strings.Join(parts, ", ")
}

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const radius = 6371000.0
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180
	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return radius * c
}

func FindCandidates(app core.App, cfg *Integration, trailID string, yearsBack int) (*CandidatesResult, error) {
	if yearsBack < 1 {
		yearsBack = 1
	}

	trail, err := app.FindRecordById("trails", trailID)
	if err != nil {
		return nil, err
	}

	gpxName := trail.GetString("gpx")
	if gpxName == "" {
		return &CandidatesResult{Candidates: []Candidate{}}, nil
	}

	fsys, err := app.NewFilesystem()
	if err != nil {
		return nil, err
	}
	defer fsys.Close()

	reader, err := fsys.GetReader(trail.BaseFilesPath() + "/" + gpxName)
	if err != nil {
		return nil, fmt.Errorf("reading gpx: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	points, err := loadTrackPointsFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parsing gpx: %w", err)
	}
	if len(points) == 0 {
		return &CandidatesResult{Candidates: []Candidate{}}, nil
	}

	baseURL, err := cfg.safeBaseURL()
	if err != nil {
		return nil, err
	}
	var takenAfter, takenBefore time.Time
	hasTimestamps := false

	boundsStart, boundsEnd, ok := timeBounds(points)
	if ok {
		hasTimestamps = true
		window := time.Duration(cfg.TimeWindowMinutes) * time.Minute
		takenAfter = boundsStart.Add(-window)
		takenBefore = boundsEnd.Add(window)
	} else {
		referenceDate := time.Now().UTC()
		if dateStr := trail.GetString("date"); len(dateStr) >= 10 {
			if t, err := time.Parse("2006-01-02", dateStr[:10]); err == nil {
				referenceDate = t.Add(24 * time.Hour)
			}
		}
		takenBefore = referenceDate
		takenAfter = referenceDate.AddDate(-yearsBack, 0, 0)
	}

	assets, err := fetchAssets(baseURL, cfg.ApiKey, takenAfter, takenBefore, cfg.MaxWaypoints*3)
	if err != nil {
		return nil, fmt.Errorf("fetching assets from immich: %w", err)
	}

	matches := matchAssets(points, assets, cfg.MaxDistanceMeters)

	candidates := make([]Candidate, 0, len(matches))
	for _, m := range matches {
		c := Candidate{
			AssetID:           m.asset.ID,
			OriginalFileName:  m.asset.OriginalFileName,
			TakenAt:           m.asset.FileCreatedAt,
			Distance:          m.distance,
			PointLat:          m.point.Lat,
			PointLon:          m.point.Lon,
			DistanceFromStart: m.point.Distance,
			City:              m.asset.ExifInfo.City,
			Country:           m.asset.ExifInfo.Country,
		}
		if m.asset.ExifInfo.Latitude != nil {
			c.Lat = *m.asset.ExifInfo.Latitude
		}
		if m.asset.ExifInfo.Longitude != nil {
			c.Lon = *m.asset.ExifInfo.Longitude
		}
		candidates = append(candidates, c)
	}

	return &CandidatesResult{
		HasTimestamps: hasTimestamps,
		Candidates:    candidates,
		HasMore:       !hasTimestamps,
		TakenAfter:    takenAfter.Format(time.RFC3339),
	}, nil
}

func FindCandidatesNearPoint(cfg *Integration, lat, lon float64, yearsBack int, doubleRadius bool) (*CandidatesResult, error) {
	if yearsBack < 1 {
		yearsBack = 1
	}
	baseURL, err := cfg.safeBaseURL()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	takenBefore := now
	takenAfter := now.AddDate(-yearsBack, 0, 0)

	assets, err := fetchAssets(baseURL, cfg.ApiKey, takenAfter, takenBefore, cfg.MaxWaypoints*3)
	if err != nil {
		return nil, fmt.Errorf("fetching assets from immich: %w", err)
	}

	threshold := float64(cfg.MaxDistanceMeters)
	if threshold <= 0 {
		threshold = 150
	}
	if doubleRadius {
		threshold *= 2
	}

	candidates := make([]Candidate, 0, len(assets))
	for _, asset := range assets {
		if asset.ExifInfo.Latitude == nil || asset.ExifInfo.Longitude == nil {
			continue
		}
		distance := haversineDistance(lat, lon, *asset.ExifInfo.Latitude, *asset.ExifInfo.Longitude)
		if distance > threshold {
			continue
		}
		candidates = append(candidates, Candidate{
			AssetID:          asset.ID,
			OriginalFileName: asset.OriginalFileName,
			TakenAt:          asset.FileCreatedAt,
			Lat:              *asset.ExifInfo.Latitude,
			Lon:              *asset.ExifInfo.Longitude,
			Distance:         distance,
			PointLat:         lat,
			PointLon:         lon,
			City:             asset.ExifInfo.City,
			Country:          asset.ExifInfo.Country,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Distance < candidates[j].Distance
	})

	hasMore := hasAssetsOlderThan(baseURL, cfg.ApiKey, takenAfter)
	return &CandidatesResult{
		HasTimestamps: false,
		Candidates:    candidates,
		HasMore:       hasMore,
		TakenAfter:    takenAfter.Format(time.RFC3339),
	}, nil
}

func hasAssetsOlderThan(baseURL, apiKey string, before time.Time) bool {
	probe := before.Add(-time.Second)
	old := probe.AddDate(-100, 0, 0)
	assets, err := fetchAssets(baseURL, apiKey, old, probe, 1)
	return err == nil && len(assets) > 0
}

func ImportAssetsToWaypoint(app core.App, cfg *Integration, userID, trailID, waypointID string, assetIDs []string) error {
	baseURL, err := cfg.safeBaseURL()
	if err != nil {
		return err
	}
	for _, assetID := range assetIDs {
		if !IsAssetID(assetID) {
			return fmt.Errorf("invalid immich asset id")
		}
		if exists, err := importedAssetExists(app, userID, assetID); err != nil {
			return err
		} else if exists {
			continue
		}
		asset, err := fetchAssetByID(baseURL, cfg.ApiKey, assetID)
		if err != nil {
			return err
		}
		if asset == nil {
			continue
		}
		var pointLat, pointLon float64
		if asset.ExifInfo.Latitude != nil {
			pointLat = *asset.ExifInfo.Latitude
		}
		if asset.ExifInfo.Longitude != nil {
			pointLon = *asset.ExifInfo.Longitude
		}
		match := assetMatch{
			asset: *asset,
			point: trackPoint{Lat: pointLat, Lon: pointLon},
		}
		photoMode := effectivePhotoModeForTrail(app, cfg.PhotoMode, trailID)
		var photo *filesystem.File
		if photoMode == PhotoModeCopy {
			photo, err = downloadAsset(baseURL, cfg.ApiKey, *asset)
			if err != nil {
				return err
			}
			if photo == nil {
				return fmt.Errorf("immich asset could not be downloaded")
			}
		}
		if err := createPhotoAsset(app, userID, trailID, waypointID, match, photoMode, photo); err != nil {
			return err
		}
	}
	return nil
}

type ImportedWaypoint struct {
	AssetID  string       `json:"assetId"`
	Waypoint *core.Record `json:"waypoint"`
}

func ImportSelectedAssets(app core.App, cfg *Integration, userID, trailID string, assetIDs []string) ([]ImportedWaypoint, error) {
	trail, err := app.FindRecordById("trails", trailID)
	if err != nil {
		return nil, err
	}

	gpxName := trail.GetString("gpx")
	if gpxName == "" {
		return nil, fmt.Errorf("trail has no gpx")
	}

	fsys, err := app.NewFilesystem()
	if err != nil {
		return nil, err
	}
	defer fsys.Close()

	reader, err := fsys.GetReader(trail.BaseFilesPath() + "/" + gpxName)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	points, err := loadTrackPointsFromBytes(data)
	if err != nil {
		return nil, err
	}

	baseURL, err := cfg.safeBaseURL()
	if err != nil {
		return nil, err
	}

	waypointCollection, err := app.FindCollectionByNameOrId("waypoints")
	if err != nil {
		return nil, err
	}

	existingWaypoints, err := app.FindRecordsByFilter("waypoints", "trail={:trail}", "", -1, 0, dbx.Params{"trail": trailID})
	if err != nil {
		return nil, err
	}

	created := make([]ImportedWaypoint, 0, len(assetIDs))

	for _, assetID := range assetIDs {
		if !IsAssetID(assetID) {
			return nil, fmt.Errorf("invalid immich asset id")
		}
		if exists, err := importedAssetExists(app, userID, assetID); err != nil {
			return nil, err
		} else if exists {
			continue
		}

		asset, err := fetchAssetByID(baseURL, cfg.ApiKey, assetID)
		if err != nil {
			return nil, err
		}
		if asset == nil || asset.ExifInfo.Latitude == nil || asset.ExifInfo.Longitude == nil {
			continue
		}

		point, distance := findNearestTrackPoint(*asset.ExifInfo.Latitude, *asset.ExifInfo.Longitude, points)
		if point == nil {
			continue
		}

		match := assetMatch{asset: *asset, point: *point, distance: distance}

		target := findWaypointByCoordinate(existingWaypoints, point.Lat, point.Lon)
		if target == nil {
			target = core.NewRecord(waypointCollection)
			target.Load(map[string]any{
				"name":                buildWaypointName(*asset),
				"lat":                 point.Lat,
				"lon":                 point.Lon,
				"icon":                "camera",
				"author":              userID,
				"distance_from_start": point.Distance,
				"trail":               trailID,
			})
			if desc := strings.TrimSpace(asset.ExifInfo.Description); desc != "" {
				target.Set("description", desc)
			}
			if err := app.Save(target); err != nil {
				return nil, err
			}
			existingWaypoints = append(existingWaypoints, target)
		}

		photoMode := effectivePhotoModeForTrail(app, cfg.PhotoMode, trailID)
		var photo *filesystem.File
		if photoMode == PhotoModeCopy {
			photo, err = downloadAsset(baseURL, cfg.ApiKey, *asset)
			if err != nil {
				return nil, err
			}
			if photo == nil {
				return nil, fmt.Errorf("immich asset could not be downloaded")
			}
		}
		if err := createPhotoAsset(app, userID, trailID, target.Id, match, photoMode, photo); err != nil {
			return nil, err
		}

		created = append(created, ImportedWaypoint{AssetID: assetID, Waypoint: target})
	}

	return created, nil
}

func fetchAssetByID(baseURL, apiKey, assetID string) (*Asset, error) {
	if !IsAssetID(assetID) {
		return nil, fmt.Errorf("invalid immich asset id")
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/assets/%s", baseURL, url.PathEscape(assetID)), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("immich asset fetch failed: %d", resp.StatusCode)
	}
	var asset Asset
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

func ProxyThumbnail(cfg *Integration, assetID string) (*http.Response, error) {
	if !IsAssetID(assetID) {
		return nil, fmt.Errorf("invalid immich asset id")
	}
	baseURL, err := cfg.safeBaseURL()
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/assets/%s/thumbnail?size=preview", baseURL, url.PathEscape(assetID))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", cfg.ApiKey)
	return httpClient.Do(req)
}
