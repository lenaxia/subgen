package integration

import "fmt"

// Sample webhook payloads based on actual media server formats

const (
	// PlexLibraryNewPayload is a Plex library.new event payload
	PlexLibraryNewPayload = `{
		"event": "library.new",
		"user": true,
		"owner": true,
		"Account": {
			"id": 1,
			"thumb": "https://plex.tv/users/1/avatar",
			"title": "Test User"
		},
		"Server": {
			"title": "Test Plex Server",
			"uuid": "abc123"
		},
		"Metadata": {
			"librarySectionType": "show",
			"ratingKey": "12345",
			"key": "/library/metadata/12345",
			"guid": "plex://episode/5d9c086fe9d5a1001f4d4c1d",
			"type": "episode",
			"title": "Test Episode",
			"grandparentTitle": "Test Show",
			"parentTitle": "Season 1",
			"index": 1,
			"parentIndex": 1,
			"year": 2024,
			"thumb": "/library/metadata/12345/thumb/1234567890",
			"addedAt": 1708012345,
			"updatedAt": 1708012345
		}
	}`

	// PlexMediaPlayPayload is a Plex media.play event payload
	PlexMediaPlayPayload = `{
		"event": "media.play",
		"user": true,
		"owner": true,
		"Account": {
			"id": 1,
			"title": "Test User"
		},
		"Metadata": {
			"ratingKey": "67890",
			"type": "episode",
			"title": "Played Episode"
		}
	}`

	// JellyfinItemAddedPayload is a Jellyfin ItemAdded event (form-encoded)
	JellyfinItemAddedPayload = `NotificationType=ItemAdded&ItemId=abc123def456&ItemType=Episode&ItemName=Test%20Episode&SeriesName=Test%20Show&SeasonNumber=1&EpisodeNumber=1`

	// JellyfinPlaybackStartPayload is a Jellyfin PlaybackStart event (form-encoded)
	JellyfinPlaybackStartPayload = `NotificationType=PlaybackStart&ItemId=xyz789abc123&ItemType=Episode&ItemName=Played%20Episode`

	// EmbyLibraryNewPayload is an Emby library.new event (form-encoded with JSON data field)
	EmbyLibraryNewPayload = `data={"Event":"library.new","Item":{"Name":"Test Episode","Path":"/media/TV/Show/S01E01.mkv","Type":"Episode","ServerId":"abc123","Id":"item123"},"Server":{"Name":"Test Emby Server","Id":"server123"}}`

	// EmbyTestNotificationPayload is an Emby test notification
	EmbyTestNotificationPayload = `data={"Event":"system.notificationtest","Server":{"Name":"Test Emby Server"}}`

	// TautulliAddedPayload is a Tautulli added event (form-encoded)
	TautulliAddedPayload = `event=added&file=/media/TV/Show/S01E01.mkv&title=Test%20Episode&show_name=Test%20Show&season_num=1&episode_num=1`

	// TautulliPlayedPayload is a Tautulli played event (form-encoded)
	TautulliPlayedPayload = `event=played&file=/media/Movies/Movie.mkv&title=Test%20Movie`
)

// GetPlexPayload returns a Plex payload with custom rating key and event
func GetPlexPayload(ratingKey string, event string) string {
	if event == "library.new" {
		return fmt.Sprintf(`{"event": "library.new", "Metadata": {"ratingKey": "%s", "type": "episode", "title": "Test Episode"}}`, ratingKey)
	}
	return fmt.Sprintf(`{"event": "media.play", "Metadata": {"ratingKey": "%s", "type": "episode", "title": "Test Episode"}}`, ratingKey)
}

// GetJellyfinPayload returns a Jellyfin payload with custom item ID and notification type
func GetJellyfinPayload(itemID string, notificationType string) string {
	return fmt.Sprintf("NotificationType=%s&ItemId=%s&ItemType=Episode&ItemName=Test%%20Episode", notificationType, itemID)
}

// GetEmbyPayload returns an Emby payload with custom file path
func GetEmbyPayload(filePath string, event string) string {
	return fmt.Sprintf(`data={"Event":"%s","Item":{"Name":"Test Episode","Path":"%s","Type":"Episode"}}`, event, filePath)
}

// GetTautulliPayload returns a Tautulli payload with custom file path and event
func GetTautulliPayload(filePath string, event string) string {
	return fmt.Sprintf("event=%s&file=%s&title=Test%%20File", event, filePath)
}
