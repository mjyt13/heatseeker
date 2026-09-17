//go:build integration

package bootstrap_test

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/ids"
)

type material struct {
	ID             string   `json:"id"`
	SubjectID      *string  `json:"subject_id"`
	UploaderID     *string  `json:"uploader_id"`
	Title          string   `json:"title"`
	Kind           string   `json:"kind"`
	Source         string   `json:"source"`
	Status         string   `json:"status"`
	TagIDs         []string `json:"tag_ids"`
	NeedsReview    bool     `json:"needs_review"`
	ReviewReason   *string  `json:"review_reason"`
	DownloadCount  int      `json:"download_count"`
	Classification struct {
		SubjectID  *string  `json:"subject_id"`
		Confidence float64  `json:"confidence"`
		Method     string   `json:"method"`
		Signals    []string `json:"signals"`
	} `json:"classification"`
	File struct {
		ID                string  `json:"id"`
		Storage           string  `json:"storage"`
		OriginalName      string  `json:"original_name"`
		Mime              string  `json:"mime"`
		SizeBytes         int64   `json:"size_bytes"`
		DriveWebViewLink  *string `json:"drive_web_view_link"`
		DriveUploadStatus *string `json:"drive_upload_status"`
		DriveUploadError  *string `json:"drive_upload_error"`
	} `json:"file"`
	Versions  []struct{ ID string } `json:"versions"`
	DrivePath []string              `json:"drive_path"`
	CanEdit   bool                  `json:"can_edit"`
	CanDelete bool                  `json:"can_delete"`
}

type page struct {
	Items      []material `json:"items"`
	NextCursor string     `json:"next_cursor"`
	InboxCount *int64     `json:"inbox_count"`
}

type openLinks struct {
	StreamURL        *string `json:"stream_url"`
	DownloadURL      *string `json:"download_url"`
	DriveWebViewLink *string `json:"drive_web_view_link"`
	Storage          string  `json:"storage"`
	Mode             string  `json:"mode"`
}

type ticket struct {
	UploadID string            `json:"upload_id"`
	Method   string            `json:"method"`
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers"`
}

func byTitle(items []material) map[string]material {
	out := map[string]material{}
	for _, m := range items {
		out[m.Title] = m
	}
	return out
}

// End-to-end stage 1: Drive folder indexing, the material feed, opening files,
// uploads to local storage, moderation and publishing uploads to Drive.
func TestMaterialsAndDrive(t *testing.T) {
	e := setup(t)
	c := e.c
	ctx := context.Background()
	runID := ids.Code(6)

	// --- people and subjects ---
	var owner, student session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Староста"}, 201, &owner)
	var created struct {
		Group map[string]any `json:"group"`
	}
	c.do("POST", "/groups", owner.AccessToken, map[string]any{"name": "Материалы " + runID}, 201, &created)
	groupID := created.Group["id"].(string)
	g := "/groups/" + groupID
	c.do("POST", "/me/credentials", owner.AccessToken, map[string]any{"email": "owner-" + runID + "@example.com", "password": "correct horse"}, 200, nil)
	c.do("POST", "/auth/register", "", map[string]any{"name": "Студент", "invite_code": created.Group["join_code"]}, 201, &student)

	var matan, bd struct{ ID string }
	c.do("POST", g+"/subjects", owner.AccessToken, map[string]any{"name": "Математический анализ", "short_name": "Матан", "aliases": []string{"матан"}}, 201, &matan)
	c.do("POST", g+"/subjects", owner.AccessToken, map[string]any{"name": "Базы данных", "short_name": "БД"}, 201, &bd)

	// --- a shared folder on (fake) Drive ---
	d := e.drive
	root := d.AddFolder("", "Магистратура")
	matanDir := d.AddFolder(root, "Матан")
	lectures := d.AddFolder(matanDir, "Лекции")
	bdDir := d.AddFolder(root, "Базы данных")
	misc := d.AddFolder(root, "Разное")
	tema1 := d.AddFile(lectures, "Tema1_Lektsia1.pdf", "application/pdf", "%PDF-1.4 lecture one")
	d.AddFile(bdDir, "Praktika_tema_1.pdf", "application/pdf", "%PDF-1.4 practice")
	doklad := d.AddFile(root, "Doklad_BD.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "PK\x03\x04 report")
	scan := d.AddFile(misc, "scan001.pdf", "application/pdf", "%PDF-1.4 scan")
	gdoc := d.AddGoogleDoc(matanDir, "Конспект")
	d.AddFile(root, "Опрос", "application/vnd.google-apps.form", "")

	var status struct {
		Configured          bool   `json:"configured"`
		ServiceAccountEmail string `json:"service_account_email"`
		UploadEnabled       bool   `json:"upload_enabled"`
		Connection          *struct {
			ID             string  `json:"id"`
			RootFolderName string  `json:"root_folder_name"`
			Status         string  `json:"status"`
			Writable       bool    `json:"writable"`
			LastError      *string `json:"last_error"`
		} `json:"connection"`
		Stats *struct {
			Files     int64 `json:"files"`
			Linked    int64 `json:"linked"`
			Skipped   int64 `json:"skipped"`
			InboxSize int64 `json:"inbox_size"`
		} `json:"stats"`
	}
	c.do("GET", g+"/drive/status", student.AccessToken, nil, 200, &status)
	if !status.Configured || status.ServiceAccountEmail == "" || status.Connection != nil || !status.UploadEnabled {
		t.Fatalf("status before connect = %+v", status)
	}

	// --- connect: only secured drive managers ---
	c.do("PUT", g+"/drive/connection", student.AccessToken, map[string]any{"folder": root}, 403, nil)
	c.do("PUT", g+"/drive/connection", owner.AccessToken, map[string]any{"folder": "https://drive.google.com/drive/folders/missingFolder123"}, 422, nil)
	c.do("PUT", g+"/drive/connection", owner.AccessToken, map[string]any{"folder": "https://example.com/nothing"}, 422, nil)
	rootRef := "https://drive.google.com/drive/u/0/folders/" + root + "?usp=sharing"
	var conn struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Writable bool   `json:"writable"`
	}
	c.do("PUT", g+"/drive/connection", owner.AccessToken, map[string]any{"folder": rootRef}, 200, &conn)
	if conn.Status != "PENDING" || !conn.Writable {
		t.Fatalf("connection = %+v", conn)
	}
	jobs := e.queue.Drain()
	if len(jobs) != 1 || jobs[0].Type != domain.JobDriveSync || !jobs[0].Payload.(domain.DriveSyncPayload).Full {
		t.Fatalf("expected a full sync job, got %+v", jobs)
	}
	connID := uuid.MustParse(conn.ID)

	// --- first (full) sync ---
	res, err := e.svc.Drive.Sync(ctx, connID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Full || res.Added != 5 || res.Skipped != 1 || res.Removed != 0 {
		t.Fatalf("first sync = %+v", res)
	}
	c.do("GET", g+"/drive/status", student.AccessToken, nil, 200, &status)
	if status.Connection == nil || status.Connection.Status != "OK" || status.Connection.RootFolderName != "Магистратура" {
		t.Fatalf("status after sync = %+v", status.Connection)
	}
	if status.Stats.Files != 6 || status.Stats.Linked != 5 || status.Stats.Skipped != 1 || status.Stats.InboxSize != 1 {
		t.Fatalf("stats = %+v", *status.Stats)
	}
	// a second full scan changes nothing
	if res, err := e.svc.Drive.Sync(ctx, connID, true); err != nil || res.Added+res.Updated+res.Removed != 0 {
		t.Fatalf("idempotent rescan = %+v, %v", res, err)
	}

	// --- the feed ---
	var all page
	c.do("GET", g+"/materials", student.AccessToken, nil, 200, &all)
	if len(all.Items) != 5 || all.InboxCount != nil {
		t.Fatalf("feed: %d items, inbox %v", len(all.Items), all.InboxCount)
	}
	items := byTitle(all.Items)
	lecture := items["Tema1 Lektsia1"]
	if lecture.SubjectID == nil || *lecture.SubjectID != matan.ID || lecture.Kind != "LECTURE" || lecture.Source != "GDRIVE" || lecture.UploaderID != nil {
		t.Fatalf("lecture = %+v", lecture)
	}
	if lecture.File.Storage != "DRIVE" || lecture.File.DriveWebViewLink == nil {
		t.Fatalf("lecture file = %+v", lecture.File)
	}
	if p := items["Praktika tema 1"]; p.SubjectID == nil || *p.SubjectID != bd.ID || p.Kind != "ASSIGNMENT" {
		t.Fatalf("practice = %+v", p)
	}
	if r := items["Doklad BD"]; r.SubjectID == nil || *r.SubjectID != bd.ID || r.Kind != "REPORT" {
		t.Fatalf("report = %+v", r)
	}
	if s := items["scan001"]; s.SubjectID != nil || !s.NeedsReview || s.ReviewReason == nil || *s.ReviewReason != "LOW_CONFIDENCE" {
		t.Fatalf("scan = %+v", s)
	}
	if k := items["Конспект"]; k.SubjectID == nil || *k.SubjectID != matan.ID || k.File.Mime != "application/vnd.google-apps.document" {
		t.Fatalf("google doc = %+v", k)
	}

	var filtered page
	c.do("GET", g+"/materials?subject_id="+matan.ID, student.AccessToken, nil, 200, &filtered)
	if len(filtered.Items) != 2 {
		t.Fatalf("subject filter: %d", len(filtered.Items))
	}
	var tags struct {
		Items []struct {
			ID        string  `json:"id"`
			SubjectID *string `json:"subject_id"`
		} `json:"items"`
	}
	c.do("GET", g+"/tags", student.AccessToken, nil, 200, &tags)
	var matanTag string
	for _, tg := range tags.Items {
		if tg.SubjectID != nil && *tg.SubjectID == matan.ID {
			matanTag = tg.ID
		}
	}
	c.do("GET", g+"/materials?tag_id="+matanTag, student.AccessToken, nil, 200, &filtered)
	if len(filtered.Items) != 2 {
		t.Fatalf("subject tag filter: %d", len(filtered.Items))
	}
	c.do("GET", g+"/materials?kind=LECTURE", student.AccessToken, nil, 200, &filtered)
	if len(filtered.Items) != 1 || filtered.Items[0].ID != lecture.ID {
		t.Fatalf("kind filter: %+v", filtered.Items)
	}
	c.do("GET", g+"/materials?q="+url.QueryEscape("praktika"), student.AccessToken, nil, 200, &filtered)
	if len(filtered.Items) != 1 {
		t.Fatalf("search: %d", len(filtered.Items))
	}
	c.do("GET", g+"/materials?q="+url.QueryEscape("конспект"), student.AccessToken, nil, 200, &filtered)
	if len(filtered.Items) != 1 {
		t.Fatalf("russian search: %d", len(filtered.Items))
	}
	c.do("GET", g+"/materials?q="+url.QueryEscape("100%_"), student.AccessToken, nil, 200, &filtered)
	if len(filtered.Items) != 0 {
		t.Fatalf("like metacharacters must be literal: %d", len(filtered.Items))
	}
	c.do("GET", g+"/materials?no_subject=true", student.AccessToken, nil, 200, &filtered)
	if len(filtered.Items) != 1 {
		t.Fatalf("no_subject: %d", len(filtered.Items))
	}
	var inbox page
	c.do("GET", g+"/materials?inbox=true", owner.AccessToken, nil, 200, &inbox)
	if len(inbox.Items) != 1 || inbox.InboxCount == nil || *inbox.InboxCount != 1 {
		t.Fatalf("inbox = %+v", inbox)
	}
	// keyset pagination walks everything exactly once
	seen := map[string]bool{}
	cursor := ""
	for pages := 0; ; pages++ {
		var p page
		c.do("GET", g+"/materials?limit=2&cursor="+cursor, student.AccessToken, nil, 200, &p)
		for _, m := range p.Items {
			if seen[m.ID] {
				t.Fatalf("duplicate %s on page %d", m.ID, pages)
			}
			seen[m.ID] = true
		}
		if p.NextCursor == "" {
			break
		}
		cursor = p.NextCursor
		if pages > 5 {
			t.Fatal("pagination does not terminate")
		}
	}
	if len(seen) != 5 {
		t.Fatalf("pagination saw %d", len(seen))
	}
	c.do("GET", g+"/materials?cursor=garbage", student.AccessToken, nil, 422, nil)

	// materials of a group are invisible to outsiders
	var outsider session
	c.do("POST", "/auth/register", "", map[string]any{"name": "Чужой"}, 201, &outsider)
	c.do("GET", "/materials/"+lecture.ID, outsider.AccessToken, nil, 404, nil)
	c.do("GET", g+"/materials", outsider.AccessToken, nil, 403, nil)

	var details material
	c.do("GET", "/materials/"+lecture.ID, student.AccessToken, nil, 200, &details)
	if strings.Join(details.DrivePath, "/") != "Матан/Лекции" || details.CanEdit || details.CanDelete || len(details.Versions) != 1 {
		t.Fatalf("details = %+v", details)
	}

	// --- activity: the initial import is summarised, not listed file by file ---
	var activity struct {
		Items []struct {
			Kind string `json:"kind"`
		} `json:"items"`
	}
	c.do("GET", g+"/activity", student.AccessToken, nil, 200, &activity)
	kinds := map[string]int{}
	for _, a := range activity.Items {
		kinds[a.Kind]++
	}
	if kinds["drive.connected"] != 1 || kinds["drive.synced"] != 1 || kinds["material.added"] != 0 {
		t.Fatalf("activity = %v", kinds)
	}
	var syncLog struct {
		Events []struct {
			Kind string `json:"kind"`
		} `json:"events"`
	}
	c.do("GET", g+"/sync?since=0", student.AccessToken, nil, 200, &syncLog)
	added := 0
	for _, ev := range syncLog.Events {
		if ev.Kind == "material.added" {
			added++
		}
	}
	if added != 5 {
		t.Fatalf("sync log has %d material.added", added)
	}

	// --- open a Drive file through the proxy ---
	var links openLinks
	c.do("GET", "/materials/"+lecture.ID+"/open", student.AccessToken, nil, 200, &links)
	if links.StreamURL == nil || links.DriveWebViewLink == nil || links.Storage != "DRIVE" || links.Mode != "CACHE" {
		t.Fatalf("open = %+v", links)
	}
	code, hdr, body := c.raw("GET", *links.StreamURL, nil, nil)
	if code != 200 || string(body) != "%PDF-1.4 lecture one" || hdr.Get("Content-Type") != "application/pdf" || !strings.HasPrefix(hdr.Get("Content-Disposition"), "inline") {
		t.Fatalf("stream: %d %v %q", code, hdr, body)
	}
	code, hdr, body = c.raw("GET", *links.StreamURL, nil, map[string]string{"Range": "bytes=0-3"})
	if code != 206 || string(body) != "%PDF" || hdr.Get("Content-Range") != "bytes 0-3/20" {
		t.Fatalf("range stream: %d %v %q", code, hdr, body)
	}
	if code, _, _ := c.raw("GET", strings.Replace(*links.StreamURL, "token=", "token=x", 1), nil, nil); code != 401 {
		t.Fatalf("tampered token: %d", code)
	}
	if code, _, _ := c.raw("GET", strings.Replace(*links.StreamURL, lecture.ID, items["scan001"].ID, 1), nil, nil); code != 401 {
		t.Fatalf("token for another material: %d", code)
	}
	var docLinks openLinks
	c.do("GET", "/materials/"+items["Конспект"].ID+"/open", student.AccessToken, nil, 200, &docLinks)
	code, hdr, _ = c.raw("GET", *docLinks.StreamURL, nil, nil)
	if code != 200 || hdr.Get("Content-Type") != "application/pdf" || !strings.Contains(hdr.Get("Content-Disposition"), ".pdf") {
		t.Fatalf("google doc export: %d %v", code, hdr)
	}
	c.do("GET", "/materials/"+lecture.ID, owner.AccessToken, nil, 200, &details)
	if details.DownloadCount != 1 {
		t.Fatalf("download count = %d", details.DownloadCount)
	}

	// --- changes on Drive are picked up incrementally ---
	d.AddFile(lectures, "Tema2_Lektsia2.pdf", "application/pdf", "%PDF-1.4 lecture two")
	d.UpdateContent(tema1, "%PDF-1.4 lecture one, fixed")
	d.Rename(doklad, "Doklad_Matan.docx")
	d.Trash(scan)
	d.Move(gdoc, misc) // leaves the subject folder: reclassified
	ivanova := d.AddFolder(root, "Ивановой")
	d.AddFile(ivanova, "notes_x.pdf", "application/pdf", "%PDF-1.4 notes")
	res, err = e.svc.Drive.Sync(ctx, connID, false)
	if err != nil {
		t.Fatal(err)
	}
	// the new folder forces a full rescan within the same run
	if !res.Full || res.Added != 2 || res.Removed != 1 {
		t.Fatalf("incremental sync = %+v", res)
	}
	c.do("GET", g+"/materials", student.AccessToken, nil, 200, &all)
	items = byTitle(all.Items)
	if len(all.Items) != 7 {
		t.Fatalf("after changes: %d items", len(all.Items))
	}
	c.do("GET", "/materials/"+lecture.ID, student.AccessToken, nil, 200, &details)
	if len(details.Versions) != 2 {
		t.Fatalf("new content must add a version, got %d", len(details.Versions))
	}
	if r, ok := items["Doklad Matan"]; !ok || r.SubjectID == nil || *r.SubjectID != matan.ID {
		t.Fatalf("renamed report = %+v (%v)", r, ok)
	}
	if s := items["scan001"]; !s.NeedsReview || s.ReviewReason == nil || *s.ReviewReason != "REMOVED_FROM_DRIVE" {
		t.Fatalf("trashed file = %+v", s)
	}
	if k := items["Конспект"]; k.SubjectID != nil || !k.NeedsReview {
		t.Fatalf("moved google doc = %+v", k)
	}
	notes := items["notes x"]
	if !notes.NeedsReview {
		t.Fatalf("notes = %+v", notes)
	}
	// incremental without structural changes stays incremental
	d.AddFile(bdDir, "Lab_2.pdf", "application/pdf", "%PDF-1.4 lab")
	if res, err = e.svc.Drive.Sync(ctx, connID, false); err != nil || res.Full || res.Added != 1 {
		t.Fatalf("plain incremental = %+v, %v", res, err)
	}
	c.do("GET", g+"/activity?limit=100", student.AccessToken, nil, 200, &activity)
	kinds = map[string]int{}
	for _, a := range activity.Items {
		kinds[a.Kind]++
	}
	if kinds["material.added"] != 3 {
		t.Fatalf("later additions must reach the activity feed: %v", kinds)
	}

	// --- Inbox: moderators classify, the folder becomes an alias ---
	c.do("POST", "/materials/"+notes.ID+"/classify", student.AccessToken, map[string]any{"subject_id": bd.ID}, 403, nil)
	var classified material
	c.do("POST", "/materials/"+notes.ID+"/classify", owner.AccessToken, map[string]any{"subject_id": bd.ID, "kind": "NOTES", "learn_alias": true}, 200, &classified)
	if classified.NeedsReview || classified.SubjectID == nil || *classified.SubjectID != bd.ID || classified.Kind != "NOTES" || classified.Classification.Method != "manual" {
		t.Fatalf("classified = %+v", classified)
	}
	var bdSubject struct {
		Aliases []string `json:"aliases"`
	}
	c.do("GET", g+"/subjects/"+bd.ID, owner.AccessToken, nil, 200, &bdSubject)
	if !contains(bdSubject.Aliases, "ивановой") {
		t.Fatalf("alias not learned: %v", bdSubject.Aliases)
	}
	d.AddFile(ivanova, "notes_y.pdf", "application/pdf", "%PDF-1.4 more notes")
	if _, err := e.svc.Drive.Sync(ctx, connID, false); err != nil {
		t.Fatal(err)
	}
	c.do("GET", g+"/materials?subject_id="+bd.ID, student.AccessToken, nil, 200, &filtered)
	if y, ok := byTitle(filtered.Items)["notes y"]; !ok || y.NeedsReview {
		t.Fatalf("learned alias not applied: %+v", filtered.Items)
	}

	// --- editing: students cannot touch Drive materials ---
	c.do("PATCH", "/materials/"+lecture.ID, student.AccessToken, map[string]any{"title": "Моё"}, 403, nil)
	var edited material
	c.do("PATCH", "/materials/"+lecture.ID, owner.AccessToken, map[string]any{"title": "Лекция 1. Пределы", "kind": "NOTES"}, 200, &edited)
	if edited.Title != "Лекция 1. Пределы" || edited.Kind != "NOTES" || edited.Classification.Method != "manual" {
		t.Fatalf("edited = %+v", edited)
	}
	c.do("PATCH", "/materials/"+lecture.ID, owner.AccessToken, map[string]any{"subject_id": uuid.NewString()}, 422, nil)
	c.do("PATCH", "/materials/"+lecture.ID, owner.AccessToken, map[string]any{"clear_subject": true}, 200, &edited)
	if edited.SubjectID != nil {
		t.Fatal("subject was not cleared")
	}

	// --- uploads to our storage ---
	var meta struct {
		Upload struct {
			MaxBytes   int64    `json:"max_bytes"`
			AllowedExt []string `json:"allowed_ext"`
		} `json:"upload"`
		Features struct {
			DriveUpload bool `json:"drive_upload"`
		} `json:"features"`
	}
	c.do("GET", "/meta", "", nil, 200, &meta)
	if meta.Upload.MaxBytes != 1<<20 || !contains(meta.Upload.AllowedExt, "pdf") || !meta.Features.DriveUpload {
		t.Fatalf("meta = %+v", meta)
	}
	pdf := []byte("%PDF-1.4 my report about " + runID)
	upload := func(token, name string, content []byte, extra map[string]any) ticket {
		t.Helper()
		body := map[string]any{"file_name": name, "size_bytes": len(content)}
		for k, v := range extra {
			body[k] = v
		}
		var tk ticket
		c.do("POST", g+"/materials/uploads", token, body, 201, &tk)
		code, _, resp := c.raw(tk.Method, tk.URL, bytes.NewReader(content), tk.Headers)
		if code != 204 {
			t.Fatalf("PUT %s: %d %s", name, code, resp)
		}
		return tk
	}
	tk := upload(student.AccessToken, "Otchet_BD.pdf", pdf, nil)
	if !strings.Contains(tk.URL, "/api/v1/media/") {
		t.Fatalf("local driver must upload through the API: %s", tk.URL)
	}
	c.do("POST", "/uploads/"+tk.UploadID+"/complete", owner.AccessToken, nil, 404, nil) // someone else's upload
	var mine material
	c.do("POST", "/uploads/"+tk.UploadID+"/complete", student.AccessToken, nil, 200, &mine)
	if mine.Source != "UPLOAD" || mine.UploaderID == nil || *mine.UploaderID != student.User.ID || mine.Kind != "REPORT" ||
		mine.SubjectID == nil || *mine.SubjectID != bd.ID || mine.File.Storage != "LOCAL" || mine.File.SizeBytes != int64(len(pdf)) {
		t.Fatalf("uploaded = %+v", mine)
	}
	var again material
	c.do("POST", "/uploads/"+tk.UploadID+"/complete", student.AccessToken, nil, 200, &again)
	if again.ID != mine.ID {
		t.Fatal("completing twice must return the same material")
	}
	jobs = e.queue.Drain()
	if len(jobs) != 1 || jobs[0].Type != domain.JobMaterialHash {
		t.Fatalf("jobs after upload = %+v", jobs)
	}
	if err := e.svc.Materials.HashVersion(ctx, uuid.MustParse(mine.File.ID)); err != nil {
		t.Fatal(err)
	}

	// rejected uploads
	c.do("POST", g+"/materials/uploads", student.AccessToken, map[string]any{"file_name": "virus.exe", "size_bytes": 10}, 422, nil)
	c.do("POST", g+"/materials/uploads", student.AccessToken, map[string]any{"file_name": "big.pdf", "size_bytes": 2 << 20}, 413, nil)
	c.do("POST", g+"/materials/uploads", outsider.AccessToken, map[string]any{"file_name": "a.pdf", "size_bytes": 10}, 403, nil)
	evil := upload(student.AccessToken, "evil.pdf", []byte("MZ\x90\x00 not a pdf"), nil)
	c.do("POST", "/uploads/"+evil.UploadID+"/complete", student.AccessToken, nil, 422, nil)
	var lying ticket
	c.do("POST", g+"/materials/uploads", student.AccessToken, map[string]any{"file_name": "small.pdf", "size_bytes": 10}, 201, &lying)
	if code, _, _ := c.raw("PUT", lying.URL, bytes.NewReader(bytes.Repeat([]byte("x"), (1<<20)+10)), nil); code != 413 {
		t.Fatalf("oversized PUT: %d", code)
	}
	c.do("POST", "/uploads/"+lying.UploadID+"/complete", student.AccessToken, nil, 422, nil)
	c.do("POST", "/uploads/"+uuid.NewString()+"/complete", student.AccessToken, nil, 404, nil)

	// opening an upload
	c.do("GET", "/materials/"+mine.ID+"/open", owner.AccessToken, nil, 200, &links)
	if links.DownloadURL == nil || links.Storage != "LOCAL" {
		t.Fatalf("open upload = %+v", links)
	}
	code, hdr, body = c.raw("GET", *links.DownloadURL, nil, nil)
	if code != 200 || !bytes.Equal(body, pdf) || hdr.Get("Content-Length") != fmt.Sprint(len(pdf)) {
		t.Fatalf("download: %d %q", code, body)
	}
	code, _, body = c.raw("GET", *links.DownloadURL, nil, map[string]string{"Range": "bytes=-6"})
	if code != 206 || string(body) != runID {
		t.Fatalf("suffix range: %d %q", code, body)
	}
	if code, _, _ := c.raw("GET", *links.DownloadURL, nil, map[string]string{"Range": "bytes=999-"}); code != 416 {
		t.Fatalf("unsatisfiable range: %d", code)
	}
	if code, _, _ := c.raw("PUT", *links.DownloadURL, strings.NewReader("overwrite"), nil); code != 401 {
		t.Fatalf("a download link must not accept uploads: %d", code)
	}

	// --- ownership rules ---
	c.do("DELETE", "/materials/"+mine.ID, student.AccessToken, nil, 403, nil) // already opened by the owner
	c.do("POST", "/materials/"+mine.ID+"/archive", student.AccessToken, nil, 204, nil)
	c.do("GET", g+"/materials?archived=true", student.AccessToken, nil, 200, &filtered)
	if len(filtered.Items) != 1 || filtered.Items[0].ID != mine.ID {
		t.Fatalf("archive list = %+v", filtered.Items)
	}
	c.do("POST", "/materials/"+mine.ID+"/restore", student.AccessToken, nil, 204, nil)
	c.do("PATCH", "/materials/"+mine.ID, student.AccessToken, map[string]any{"title": "Отчёт по БД", "tag_ids": []string{matanTag}}, 200, &edited)
	if edited.Title != "Отчёт по БД" || len(edited.TagIDs) != 0 {
		t.Fatalf("subject tags must not be attached: %+v", edited)
	}
	var topic struct{ ID string }
	c.do("POST", g+"/tags", owner.AccessToken, map[string]any{"name": "Экзамен", "kind": "TOPIC"}, 201, &topic)
	c.do("PATCH", "/materials/"+mine.ID, student.AccessToken, map[string]any{"tag_ids": []string{topic.ID}}, 200, &edited)
	if len(edited.TagIDs) != 1 {
		t.Fatalf("tags = %v", edited.TagIDs)
	}
	c.do("GET", g+"/materials?tag_id="+topic.ID+"&mine=true", student.AccessToken, nil, 200, &filtered)
	if len(filtered.Items) != 1 || filtered.Items[0].ID != mine.ID {
		t.Fatalf("tag + mine filter = %+v", filtered.Items)
	}

	fresh := upload(student.AccessToken, "draft.txt", []byte("черновик"), nil)
	var draft material
	c.do("POST", "/uploads/"+fresh.UploadID+"/complete", student.AccessToken, nil, 200, &draft)
	c.do("DELETE", "/materials/"+draft.ID, student.AccessToken, nil, 204, nil) // nobody opened it yet
	c.do("GET", "/materials/"+draft.ID, student.AccessToken, nil, 404, nil)
	c.do("GET", "/materials/"+draft.ID, owner.AccessToken, nil, 200, &details)
	if details.Status != "DELETED" {
		t.Fatalf("moderator sees %s", details.Status)
	}
	c.do("POST", "/materials/"+draft.ID+"/restore", student.AccessToken, nil, 404, nil)
	c.do("POST", "/materials/"+draft.ID+"/restore", owner.AccessToken, nil, 204, nil)
	c.do("DELETE", "/materials/"+mine.ID, owner.AccessToken, nil, 204, nil) // moderators may delete anything

	// --- publishing an upload to Drive (method A) ---
	c.do("POST", g+"/materials/uploads", student.AccessToken, map[string]any{"file_name": "a.pdf", "size_bytes": 10, "to_drive": true}, 403, nil)
	c.do("POST", "/me/credentials", student.AccessToken, map[string]any{"email": "student-" + runID + "@example.com", "password": "correct horse"}, 200, nil)
	e.queue.Drain()
	pub := upload(student.AccessToken, "Lektsia_3_matan.pdf", []byte("%PDF-1.4 published"), map[string]any{"to_drive": true, "subject_id": matan.ID})
	var published material
	c.do("POST", "/uploads/"+pub.UploadID+"/complete", student.AccessToken, nil, 200, &published)
	if published.File.DriveUploadStatus == nil || *published.File.DriveUploadStatus != "PENDING" {
		t.Fatalf("published = %+v", published.File)
	}
	jobs = e.queue.Drain()
	if len(jobs) != 2 || jobs[1].Type != domain.JobDriveUpload {
		t.Fatalf("jobs after publishing = %+v", jobs)
	}
	if err := e.svc.Drive.UploadVersion(ctx, uuid.MustParse(published.ID), uuid.MustParse(published.File.ID)); err != nil {
		t.Fatal(err)
	}
	fileID, ok := d.FindByName("Lektsia_3_matan.pdf")
	if !ok {
		t.Fatal("file was not created on Drive")
	}
	onDrive, _ := d.GetFile(ctx, fileID)
	if onDrive.Parent() != matanDir || onDrive.AppProperties["heatseeker_material_id"] != published.ID || !strings.Contains(onDrive.Description, "Студент") {
		t.Fatalf("drive file = %+v", onDrive)
	}
	c.do("GET", "/materials/"+published.ID, student.AccessToken, nil, 200, &details)
	if details.File.DriveUploadStatus == nil || *details.File.DriveUploadStatus != "DONE" || details.File.DriveWebViewLink == nil || details.File.Storage != "LOCAL" {
		t.Fatalf("after publishing = %+v", details.File)
	}
	before := countMaterials(t, c, g, owner.AccessToken)
	if _, err := e.svc.Drive.Sync(ctx, connID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Drive.Sync(ctx, connID, true); err != nil {
		t.Fatal(err)
	}
	if after := countMaterials(t, c, g, owner.AccessToken); after != before {
		t.Fatalf("published file was indexed as a duplicate: %d → %d", before, after)
	}
	// a service account without storage quota fails permanently with a hint
	d.QuotaExceeded = true
	failing := upload(student.AccessToken, "second.pdf", []byte("%PDF-1.4 second"), map[string]any{"to_drive": true})
	var second material
	c.do("POST", "/uploads/"+failing.UploadID+"/complete", student.AccessToken, nil, 200, &second)
	if err := e.svc.Drive.UploadVersion(ctx, uuid.MustParse(second.ID), uuid.MustParse(second.File.ID)); err == nil {
		t.Fatal("expected a permanent failure")
	}
	c.do("GET", "/materials/"+second.ID, student.AccessToken, nil, 200, &details)
	if details.File.DriveUploadStatus == nil || *details.File.DriveUploadStatus != "FAILED" || details.File.DriveUploadError == nil || !strings.Contains(*details.File.DriveUploadError, "shared drive") {
		t.Fatalf("quota failure = %+v", details.File)
	}

	// --- purge: deleted Drive materials do not come back ---
	c.do("DELETE", "/materials/"+items["Praktika tema 1"].ID, owner.AccessToken, nil, 204, nil)
	if _, err := e.svc.Materials.PurgeDeleted(ctx); err != nil {
		t.Fatal(err)
	}
	c.do("GET", "/materials/"+items["Praktika tema 1"].ID, owner.AccessToken, nil, 404, nil)
	c.do("GET", "/materials/"+mine.ID, owner.AccessToken, nil, 404, nil)
	if _, err := e.svc.Drive.Sync(ctx, connID, true); err != nil {
		t.Fatal(err)
	}
	c.do("GET", g+"/materials?q=praktika", owner.AccessToken, nil, 200, &filtered)
	if len(filtered.Items) != 0 {
		t.Fatalf("purged Drive file came back: %+v", filtered.Items)
	}

	// --- items listing and disconnect ---
	var driveItems struct {
		Items []struct {
			State string `json:"state"`
		} `json:"items"`
	}
	c.do("GET", g+"/drive/items?state=SKIPPED", owner.AccessToken, nil, 200, &driveItems)
	if len(driveItems.Items) != 2 {
		t.Fatalf("skipped items = %+v", driveItems.Items)
	}
	c.do("GET", g+"/drive/items", student.AccessToken, nil, 403, nil)
	c.do("POST", g+"/drive/sync", student.AccessToken, map[string]any{"full": true}, 403, nil)
	e.queue.Drain()
	c.do("POST", g+"/drive/sync", owner.AccessToken, map[string]any{"full": true}, 202, nil)
	if jobs := e.queue.Drain(); len(jobs) != 1 || jobs[0].Type != domain.JobDriveSync {
		t.Fatalf("manual sync jobs = %+v", jobs)
	}
	c.do("DELETE", g+"/drive/connection", student.AccessToken, nil, 403, nil)
	c.do("DELETE", g+"/drive/connection", owner.AccessToken, nil, 204, nil)
	c.do("GET", g+"/drive/status", owner.AccessToken, nil, 200, &status)
	if status.Connection != nil {
		t.Fatal("connection still present")
	}
	if n := countMaterials(t, c, g, owner.AccessToken); n == 0 {
		t.Fatal("materials must survive a disconnect")
	}
	if res, err := e.svc.Drive.Sync(ctx, connID, false); err != nil || res.Added != 0 {
		t.Fatalf("sync of a removed connection = %+v, %v", res, err)
	}
}

func countMaterials(t *testing.T, c *client, g, token string) int {
	t.Helper()
	var p page
	c.do("GET", g+"/materials?limit=100", token, nil, 200, &p)
	return len(p.Items)
}
