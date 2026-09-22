//go:build integration

package bootstrap_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/url"
	"strings"
	"testing"
	"time"

	"heatseeker/api/internal/platform/ids"
)

type occurrenceDTO struct {
	EventID         string     `json:"event_id"`
	Date            string     `json:"date"`
	StartsAt        time.Time  `json:"starts_at"`
	EndsAt          time.Time  `json:"ends_at"`
	PlannedStartsAt *time.Time `json:"planned_starts_at"`
	Title           string     `json:"title"`
	Location        string     `json:"location"`
	Status          string     `json:"status"`
	ChangeNote      string     `json:"change_note"`
	Recurring       bool       `json:"recurring"`
	Version         int32      `json:"version"`
}

type scheduleEventDTO struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version int32  `json:"version"`
	Until   string `json:"until"`
	Repeat  *struct {
		IntervalWeeks int      `json:"interval_weeks"`
		Weekdays      []string `json:"weekdays"`
	} `json:"repeat"`
	Exceptions []struct {
		Date string `json:"date"`
		Kind string `json:"kind"`
	} `json:"exceptions"`
}

// The schedule end to end: a biweekly series, cancelling, moving and putting
// back single classes, editing "from this date on", deleting the rest of a
// series, and the calendar feed.
func TestScheduleFlow(t *testing.T) {
	e := setup(t)
	c := e.c
	runID := ids.Code(6)
	msk, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}

	var owner, student session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Староста"}, 201, &owner)
	var created struct {
		Group map[string]any `json:"group"`
	}
	c.do("POST", "/groups", owner.AccessToken, map[string]any{"name": "Расписание ИС-24"}, 201, &created)
	groupID := created.Group["id"].(string)
	c.do("POST", "/auth/register", "", map[string]any{"name": "Студент", "invite_code": created.Group["join_code"]}, 201, &student)
	var subject struct {
		ID string `json:"id"`
	}
	c.do("POST", "/groups/"+groupID+"/subjects", owner.AccessToken, map[string]any{"name": "Матан"}, 201, &subject)

	window := func(token string, from, to time.Time) []occurrenceDTO {
		t.Helper()
		var out struct {
			Items []occurrenceDTO `json:"items"`
		}
		q := url.Values{"from": {from.Format(time.RFC3339)}, "to": {to.Format(time.RFC3339)}}
		c.do("GET", "/groups/"+groupID+"/schedule?"+q.Encode(), token, nil, 200, &out)
		return out.Items
	}
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, msk) }

	// Mondays 10:00–11:30, every other week, from 2026-09-07 to 2026-12-28.
	series := map[string]any{
		"client_id": "33333333-3333-4333-8333-333333333333", "subject_id": subject.ID, "kind": "SEMINAR",
		"starts_at": day(9, 7).Add(10 * time.Hour).Format(time.RFC3339),
		"ends_at":   day(9, 7).Add(11*time.Hour + 30*time.Minute).Format(time.RFC3339),
		"timezone":  "Europe/Moscow", "location": "301",
		"repeat": map[string]any{"interval_weeks": 2}, "until": "2026-12-28",
	}
	// Editing needs the headman or an admin with a secured account.
	c.do("POST", "/groups/"+groupID+"/schedule/events", student.AccessToken, series, 403, nil)
	c.do("POST", "/groups/"+groupID+"/schedule/events", owner.AccessToken, series, 403, nil)
	c.do("POST", "/me/credentials", owner.AccessToken, map[string]any{"email": "sched-" + runID + "@example.com", "password": "correct horse"}, 200, nil)

	var ev scheduleEventDTO
	c.do("POST", "/groups/"+groupID+"/schedule/events", owner.AccessToken, series, 201, &ev)
	if ev.Repeat == nil || ev.Repeat.IntervalWeeks != 2 || ev.Until != "2026-12-28" || ev.Version != 1 {
		t.Fatalf("created = %+v", ev)
	}
	var again scheduleEventDTO
	c.do("POST", "/groups/"+groupID+"/schedule/events", owner.AccessToken, series, 201, &again)
	if again.ID != ev.ID {
		t.Fatal("repeat with the same client_id created another event")
	}
	bad := map[string]any{"title": "x", "starts_at": series["starts_at"], "ends_at": series["starts_at"], "timezone": "Europe/Moscow"}
	c.do("POST", "/groups/"+groupID+"/schedule/events", owner.AccessToken, bad, 422, nil)
	bad["ends_at"], bad["timezone"] = series["ends_at"], "Mars/Olympus"
	c.do("POST", "/groups/"+groupID+"/schedule/events", owner.AccessToken, bad, 422, nil)

	sept := window(student.AccessToken, day(9, 1), day(10, 1))
	if len(sept) != 2 || sept[0].Date != "2026-09-07" || sept[1].Date != "2026-09-21" || !sept[0].Recurring {
		t.Fatalf("september = %+v", sept)
	}
	if h := sept[1].StartsAt.In(msk).Hour(); h != 10 || sept[1].Location != "301" {
		t.Fatalf("class = %+v", sept[1])
	}

	// --- single classes: cancel, move, put back ---
	occ := "/schedule/events/" + ev.ID + "/occurrences/"
	c.do("PUT", occ+"2026-09-07", student.AccessToken, map[string]any{"cancelled": true}, 403, nil)
	c.do("PUT", occ+"2026-09-14", owner.AccessToken, map[string]any{"cancelled": true}, 422, nil) // no class that day
	var cancelled occurrenceDTO
	c.do("PUT", occ+"2026-09-07", owner.AccessToken, map[string]any{"cancelled": true, "note": "праздник"}, 200, &cancelled)
	if cancelled.Status != "CANCELLED" || cancelled.ChangeNote != "праздник" {
		t.Fatalf("cancelled = %+v", cancelled)
	}
	moved := day(10, 1).Add(14 * time.Hour)
	var changed occurrenceDTO
	c.do("PUT", occ+"2026-09-21", owner.AccessToken, map[string]any{
		"starts_at": moved.Format(time.RFC3339), "ends_at": moved.Add(90 * time.Minute).Format(time.RFC3339), "location": "Онлайн",
	}, 200, &changed)
	if changed.Status != "CHANGED" || changed.PlannedStartsAt == nil || changed.Location != "Онлайн" {
		t.Fatalf("changed = %+v", changed)
	}
	c.do("PUT", occ+"2026-09-21", owner.AccessToken, map[string]any{}, 422, nil) // nothing to change
	var one occurrenceDTO
	c.do("GET", occ+"2026-09-21", student.AccessToken, nil, 200, &one)
	if one.Status != "CHANGED" || !one.StartsAt.Equal(moved) {
		t.Fatalf("one class = %+v", one)
	}
	c.do("GET", occ+"2026-09-14", student.AccessToken, nil, 404, nil)

	sept = window(student.AccessToken, day(9, 1), day(9, 30))
	if len(sept) != 1 || sept[0].Status != "CANCELLED" {
		t.Fatalf("september after changes = %+v", sept)
	}
	early := window(student.AccessToken, day(10, 1), day(10, 3))
	if len(early) != 1 || early[0].Date != "2026-09-21" || !early[0].StartsAt.Equal(moved) {
		t.Fatalf("moved class = %+v", early)
	}
	var reset occurrenceDTO
	c.do("DELETE", occ+"2026-09-07", owner.AccessToken, nil, 200, &reset)
	if reset.Status != "SCHEDULED" {
		t.Fatalf("reset = %+v", reset)
	}
	c.do("DELETE", occ+"2026-09-07", owner.AccessToken, nil, 404, nil)

	// --- from 2026-10-05 on the class moves to room 405 ---
	edit := map[string]any{}
	for k, v := range series {
		edit[k] = v
	}
	delete(edit, "client_id")
	edit["location"], edit["version"], edit["scope"], edit["from"] = "405", ev.Version, "FOLLOWING", "2026-10-05"
	edit["starts_at"] = day(10, 5).Add(10 * time.Hour).Format(time.RFC3339)
	edit["ends_at"] = day(10, 5).Add(11*time.Hour + 30*time.Minute).Format(time.RFC3339)
	var next scheduleEventDTO
	c.do("PATCH", "/schedule/events/"+ev.ID, owner.AccessToken, edit, 200, &next)
	if next.ID == ev.ID || next.Version != 1 {
		t.Fatalf("split = %+v", next)
	}
	c.do("PATCH", "/schedule/events/"+ev.ID, owner.AccessToken, edit, 409, nil) // stale version
	var old scheduleEventDTO
	c.do("GET", "/schedule/events/"+ev.ID, student.AccessToken, nil, 200, &old)
	if old.Until != "2026-10-04" || len(old.Exceptions) != 1 || old.Exceptions[0].Date != "2026-09-21" {
		t.Fatalf("old part = %+v", old)
	}
	autumn := window(student.AccessToken, day(9, 1), day(11, 1))
	var rooms []string
	for _, o := range autumn {
		rooms = append(rooms, o.Date+" "+o.Location)
	}
	if got := strings.Join(rooms, ", "); got != "2026-09-07 301, 2026-09-21 Онлайн, 2026-10-05 405, 2026-10-19 405" {
		t.Fatalf("autumn = %s", got)
	}

	// --- the rest of the new part is dropped from 2026-11-02 ---
	c.do("DELETE", "/schedule/events/"+next.ID+"?version=1&scope=FOLLOWING&from=2026-11-02", owner.AccessToken, nil, 204, nil)
	if nov := window(student.AccessToken, day(11, 1), day(12, 1)); len(nov) != 0 {
		t.Fatalf("november = %+v", nov)
	}
	// A one-off exam, then the whole of it deleted.
	var exam scheduleEventDTO
	c.do("POST", "/groups/"+groupID+"/schedule/events", owner.AccessToken, map[string]any{
		"title": "Экзамен", "kind": "EXAM", "timezone": "Europe/Moscow",
		"starts_at": day(12, 20).Add(9 * time.Hour).Format(time.RFC3339), "ends_at": day(12, 20).Add(12 * time.Hour).Format(time.RFC3339),
	}, 201, &exam)
	if dec := window(student.AccessToken, day(12, 1), day(12, 31)); len(dec) != 1 || dec[0].Recurring {
		t.Fatalf("december = %+v", dec)
	}
	c.do("DELETE", "/schedule/events/"+exam.ID+"?version="+"1", owner.AccessToken, nil, 204, nil)
	c.do("GET", "/schedule/events/"+exam.ID, student.AccessToken, nil, 404, nil)

	// The window is bounded.
	c.do("GET", "/groups/"+groupID+"/schedule?"+url.Values{
		"from": {day(1, 1).Format(time.RFC3339)}, "to": {day(12, 31).Format(time.RFC3339)},
	}.Encode(), student.AccessToken, nil, 422, nil)

	// --- calendar feed by a personal link ---
	var link struct {
		URL string `json:"url"`
	}
	c.do("GET", "/groups/"+groupID+"/schedule/calendar-link", student.AccessToken, nil, 200, &link)
	status, header, body := c.raw("GET", link.URL, nil, nil)
	if status != 200 || !strings.HasPrefix(header.Get("Content-Type"), "text/calendar") {
		t.Fatalf("calendar: %d %s", status, header.Get("Content-Type"))
	}
	ics := string(body)
	for _, want := range []string{"BEGIN:VCALENDAR", "RRULE:FREQ=WEEKLY;INTERVAL=2;UNTIL=", "SUMMARY:Матан · семинар", "LOCATION:405"} {
		if !strings.Contains(ics, want) {
			t.Errorf("calendar lacks %q:\n%s", want, ics)
		}
	}
	if status, _, _ := c.raw("GET", strings.Replace(link.URL, "token=", "token=x", 1), nil, nil); status != 401 {
		t.Fatalf("tampered calendar link: %d", status)
	}

	// The activity feed tells the group what changed.
	var activity struct {
		Items []struct {
			Kind string `json:"kind"`
		} `json:"items"`
	}
	c.do("GET", "/groups/"+groupID+"/activity", student.AccessToken, nil, 200, &activity)
	kinds := map[string]bool{}
	for _, a := range activity.Items {
		kinds[a.Kind] = true
	}
	for _, k := range []string{"schedule.created", "schedule.cancelled", "schedule.changed", "schedule.reset", "schedule.updated", "schedule.deleted"} {
		if !kinds[k] {
			t.Errorf("activity lacks %s", k)
		}
	}
}

// A picture gets a small upright JPEG by the link in its version.
func TestPictureThumbnail(t *testing.T) {
	e := setup(t)
	c := e.c
	var owner session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Фотограф"}, 201, &owner)
	var created struct {
		Group map[string]any `json:"group"`
	}
	c.do("POST", "/groups", owner.AccessToken, map[string]any{"name": "Фото ИС-24"}, 201, &created)
	groupID := created.Group["id"].(string)

	upload := func(name string, content []byte) string {
		t.Helper()
		var tk struct {
			UploadID string            `json:"upload_id"`
			Method   string            `json:"method"`
			URL      string            `json:"url"`
			Headers  map[string]string `json:"headers"`
		}
		c.do("POST", "/groups/"+groupID+"/materials/uploads", owner.AccessToken,
			map[string]any{"file_name": name, "size_bytes": len(content)}, 201, &tk)
		if code, _, resp := c.raw(tk.Method, tk.URL, bytes.NewReader(content), tk.Headers); code != 204 {
			t.Fatalf("PUT %s: %d %s", name, code, resp)
		}
		var m struct {
			ID string `json:"id"`
		}
		c.do("POST", "/uploads/"+tk.UploadID+"/complete", owner.AccessToken, nil, 200, &m)
		return m.ID
	}

	img := image.NewRGBA(image.Rect(0, 0, 1600, 800))
	for y := 0; y < 800; y++ {
		for x := 0; x < 1600; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	pictureID := upload("доска.png", buf.Bytes())
	pdfID := upload("задание.pdf", []byte("%PDF-1.4 not a picture"))

	type material struct {
		File struct {
			ThumbnailURL *string `json:"thumbnail_url"`
		} `json:"file"`
	}
	var pdf material
	c.do("GET", "/materials/"+pdfID, owner.AccessToken, nil, 200, &pdf)
	if pdf.File.ThumbnailURL != nil {
		t.Fatalf("a PDF has a thumbnail: %s", *pdf.File.ThumbnailURL)
	}
	var picture material
	c.do("GET", "/materials/"+pictureID, owner.AccessToken, nil, 200, &picture)
	if picture.File.ThumbnailURL == nil {
		t.Fatal("a picture has no thumbnail link")
	}
	for i := 0; i < 2; i++ { // made, then served from storage
		status, header, body := c.raw("GET", *picture.File.ThumbnailURL, nil, nil)
		if status != 200 || header.Get("Content-Type") != "image/jpeg" || !strings.Contains(header.Get("Cache-Control"), "immutable") {
			t.Fatalf("thumbnail #%d: %d %v", i, status, header)
		}
		thumb, err := jpeg.Decode(bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if b := thumb.Bounds(); b.Dx() != 640 || b.Dy() != 320 {
			t.Fatalf("thumbnail size = %v", b.Size())
		}
	}
	if status, _, _ := c.raw("GET", strings.Replace(*picture.File.ThumbnailURL, "token=", "token=x", 1), nil, nil); status != 401 {
		t.Fatalf("tampered thumbnail link: %d", status)
	}
}
