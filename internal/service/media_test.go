package service

import "testing"

func TestAggregateEpisodeSeriesGroupsRepeatedEpisodesInPlace(t *testing.T) {
	items := []MediaItem{
		{
			ID:          "episode-old",
			Name:        "Arrival",
			Type:        "Episode",
			SeriesID:    "series-a",
			SeriesName:  "North Star",
			SeasonName:  "Season 1",
			IndexNumber: 2,
			ImageURL:    "old.jpg",
			Playable:    true,
			UserData:    &UserData{LastPlayedDate: "2026-01-01T10:00:00Z", IsFavorite: true},
		},
		{ID: "movie", Name: "A Film", Type: "Movie", Playable: true},
		{
			ID:              "episode-new",
			Name:            "Return",
			Type:            "Episode",
			SeriesID:        "series-a",
			SeriesName:      "North Star",
			SeasonName:      "Season 2",
			IndexNumber:     3,
			ImageURL:        "new.jpg",
			SeriesImageURL:  "series.jpg",
			SeriesImageHigh: "series-high.jpg",
			Playable:        true,
			UserData:        &UserData{LastPlayedDate: "2026-02-01T10:00:00Z", IsFavorite: true},
		},
		{ID: "orphan", Name: "Special", Type: "Episode", Playable: true},
		{ID: "single", Name: "Pilot", Type: "Episode", SeriesID: "series-b", SeriesName: "Solo", Playable: true},
	}

	got := aggregateEpisodeSeries(items)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4: %#v", len(got), got)
	}

	group := got[0]
	if group.ID != "series-a" || group.Type != "Series" || group.Name != "North Star" {
		t.Fatalf("unexpected group identity: %#v", group)
	}
	if group.EpisodeCount != 2 || group.LatestEpisodeName != "Season 2 · E03 · Return" {
		t.Fatalf("unexpected group summary: count=%d latest=%q", group.EpisodeCount, group.LatestEpisodeName)
	}
	if group.ImageURL != "series.jpg" || group.ImageURLHigh != "series-high.jpg" {
		t.Fatalf("group images = %q / %q, want series artwork", group.ImageURL, group.ImageURLHigh)
	}
	if group.Playable || !group.Browsable {
		t.Fatalf("group must browse instead of play: playable=%v browsable=%v", group.Playable, group.Browsable)
	}
	if got[1].ID != "movie" || got[2].ID != "orphan" || got[3].ID != "single" {
		t.Fatalf("independent item order changed: %#v", got)
	}
	if got[2].EpisodeCount != 0 || got[3].EpisodeCount != 0 {
		t.Fatalf("orphan and singleton episodes must stay direct entries: %#v", got)
	}
}

func TestAggregateEpisodeSeriesPreservesFirstGroupOccurrence(t *testing.T) {
	items := []MediaItem{
		{ID: "a1", Name: "A1", Type: "Episode", SeriesID: "a", SeriesName: "Alpha"},
		{ID: "b1", Name: "B1", Type: "Episode", SeriesID: "b", SeriesName: "Beta"},
		{ID: "a2", Name: "A2", Type: "Episode", SeriesID: "a", SeriesName: "Alpha"},
		{ID: "movie", Name: "Movie", Type: "Movie"},
		{ID: "b2", Name: "B2", Type: "Episode", SeriesID: "b", SeriesName: "Beta"},
	}

	got := aggregateEpisodeSeries(items)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" || got[2].ID != "movie" {
		t.Fatalf("order = [%s %s %s], want [a b movie]", got[0].ID, got[1].ID, got[2].ID)
	}
	if got[0].EpisodeCount != 2 || got[1].EpisodeCount != 2 {
		t.Fatalf("unexpected group counts: %d, %d", got[0].EpisodeCount, got[1].EpisodeCount)
	}
}

func TestAggregateEpisodeSeriesUsesFirstEpisodeWithoutPlaybackDates(t *testing.T) {
	items := []MediaItem{
		{ID: "first", Name: "First", Type: "Episode", SeriesID: "series", SeriesName: "Series", ImageURL: "first.jpg"},
		{ID: "second", Name: "Second", Type: "Episode", SeriesID: "series", SeriesName: "Series", ImageURL: "second.jpg"},
	}

	got := aggregateEpisodeSeries(items)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ImageURL != "first.jpg" || got[0].LatestEpisodeName != "First" {
		t.Fatalf("fallback representative must be the first API item: %#v", got[0])
	}
}
