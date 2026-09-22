//go:build integration

package bootstrap_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
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
	LearnedAlias   string   `json:"learned_alias"`
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
		PreviewStatus     string  `json:"preview_status"`
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
	PreviewURL       *string `json:"preview_url"`
	PreviewStatus    string  `json:"preview_status"`
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
			LastErrorCode  *string `json:"last_error_code"`
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
	var problem struct {
		Type string `json:"type"`
	}
	c.do("PUT", g+"/drive/connection", owner.AccessToken, map[string]any{"folder": "https://drive.google.com/drive/folders/missingFolder123"}, 422, &problem)
	if problem.Type != domain.ErrorPrefix+domain.CodeFolderNotShared {
		t.Fatalf("missing folder: type %q", problem.Type)
	}
	c.do("PUT", g+"/drive/connection", owner.AccessToken, map[string]any{"folder": "https://example.com/nothing"}, 422, &problem)
	if problem.Type != domain.ErrorPrefix+domain.CodeFolderLink {
		t.Fatalf("bad link: type %q", problem.Type)
	}
	c.do("PUT", g+"/drive/connection", owner.AccessToken, map[string]any{"folder": tema1}, 422, &problem)
	if problem.Type != domain.ErrorPrefix+domain.CodeNotAFolder {
		t.Fatalf("file link: type %q", problem.Type)
	}
	// uncoded errors keep the default problem type
	c.do("PUT", g+"/drive/connection", student.AccessToken, map[string]any{"folder": root}, 403, &problem)
	if problem.Type != "" && problem.Type != "about:blank" {
		t.Fatalf("uncoded error: type %q", problem.Type)
	}
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
	checkOldDriveVersion(t, e, lecture.ID, details.Versions[1].ID, tema1, student.AccessToken)
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
	d.AddFile(ivanova, "notes_y.pdf", "application/pdf", "%PDF-1.4 more notes")
	if _, err := e.svc.Drive.Sync(ctx, connID, false); err != nil {
		t.Fatal(err)
	}
	e.queue.Drain()
	c.do("POST", "/materials/"+notes.ID+"/classify", student.AccessToken, map[string]any{"subject_id": bd.ID}, 403, nil)
	var classified material
	c.do("POST", "/materials/"+notes.ID+"/classify", owner.AccessToken, map[string]any{"subject_id": bd.ID, "kind": "NOTES", "learn_alias": true}, 200, &classified)
	if classified.NeedsReview || classified.SubjectID == nil || *classified.SubjectID != bd.ID || classified.Kind != "NOTES" ||
		classified.Classification.Method != "manual" || classified.LearnedAlias != "ивановой" {
		t.Fatalf("classified = %+v", classified)
	}
	var bdSubject struct {
		Aliases []string `json:"aliases"`
	}
	c.do("GET", g+"/subjects/"+bd.ID, owner.AccessToken, nil, 200, &bdSubject)
	if !contains(bdSubject.Aliases, "ивановой") {
		t.Fatalf("alias not learned: %v", bdSubject.Aliases)
	}
	// the learned alias sorts the sibling that was already waiting in the Inbox
	reclassify := func() int {
		t.Helper()
		jobs := e.queue.Drain()
		if len(jobs) != 1 || jobs[0].Type != domain.JobMaterialsReclassify ||
			jobs[0].Payload.(domain.GroupPayload).GroupID != groupID {
			t.Fatalf("expected one reclassify job, got %+v", jobs)
		}
		n, err := e.svc.Drive.Reclassify(ctx, uuid.MustParse(groupID))
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := reclassify(); n != 1 {
		t.Fatalf("reclassified %d, want 1", n)
	}
	c.do("GET", g+"/materials?subject_id="+bd.ID, student.AccessToken, nil, 200, &filtered)
	if y, ok := byTitle(filtered.Items)["notes y"]; !ok || y.NeedsReview || y.Classification.Method != "auto" {
		t.Fatalf("learned alias not applied to the waiting file: %+v", filtered.Items)
	}
	// ...and new files in that folder during sync
	d.AddFile(ivanova, "notes_z.pdf", "application/pdf", "%PDF-1.4 even more notes")
	if _, err := e.svc.Drive.Sync(ctx, connID, false); err != nil {
		t.Fatal(err)
	}
	c.do("GET", g+"/materials?subject_id="+bd.ID, student.AccessToken, nil, 200, &filtered)
	if z, ok := byTitle(filtered.Items)["notes z"]; !ok || z.NeedsReview {
		t.Fatalf("learned alias not applied during sync: %+v", filtered.Items)
	}
	// a new subject sorts files that waited for it
	econ := d.AddFolder(root, "Эконометрика")
	d.AddFile(econ, "hw1.pdf", "application/pdf", "%PDF-1.4 homework")
	if _, err := e.svc.Drive.Sync(ctx, connID, false); err != nil {
		t.Fatal(err)
	}
	e.queue.Drain()
	var econSubject struct{ ID string }
	c.do("POST", g+"/subjects", owner.AccessToken, map[string]any{"name": "Эконометрика"}, 201, &econSubject)
	if n := reclassify(); n != 1 {
		t.Fatalf("reclassified %d after new subject, want 1", n)
	}
	if n, err := e.svc.Drive.Reclassify(ctx, uuid.MustParse(groupID)); err != nil || n != 0 {
		t.Fatalf("second run must be a no-op: %d, %v", n, err)
	}
	c.do("GET", g+"/materials?subject_id="+econSubject.ID, student.AccessToken, nil, 200, &filtered)
	if len(filtered.Items) != 1 || filtered.Items[0].NeedsReview {
		t.Fatalf("new subject not applied: %+v", filtered.Items)
	}
	c.do("GET", g+"/activity?limit=100", owner.AccessToken, nil, 200, &activity)
	kinds = map[string]int{}
	for _, a := range activity.Items {
		kinds[a.Kind]++
	}
	if kinds["drive.reclassified"] != 2 {
		t.Fatalf("reclassification must be summarised in the activity feed: %v", kinds)
	}

	// --- Inbox bulk action ---
	var waiting page
	c.do("GET", g+"/materials?inbox=true", owner.AccessToken, nil, 200, &waiting)
	if len(waiting.Items) != 2 { // the moved Google Doc and the trashed scan
		t.Fatalf("inbox before bulk = %+v", waiting.Items)
	}
	bulkIDs := []string{waiting.Items[0].ID, waiting.Items[1].ID, waiting.Items[0].ID, uuid.NewString()}
	c.do("POST", g+"/materials/classify", student.AccessToken, map[string]any{"material_ids": bulkIDs, "subject_id": matan.ID}, 403, nil)
	tooMany := make([]string, 201)
	for i := range tooMany {
		tooMany[i] = uuid.NewString()
	}
	c.do("POST", g+"/materials/classify", owner.AccessToken, map[string]any{"material_ids": tooMany}, 422, nil)
	c.do("POST", g+"/materials/classify", owner.AccessToken, map[string]any{"material_ids": bulkIDs, "subject_id": uuid.NewString()}, 422, nil)
	var bulk struct {
		Classified int `json:"classified"`
	}
	c.do("POST", g+"/materials/classify", owner.AccessToken, map[string]any{"material_ids": bulkIDs, "subject_id": matan.ID, "kind": "NOTES"}, 200, &bulk)
	if bulk.Classified != 2 {
		t.Fatalf("bulk classified %d, want 2 (duplicates and unknown ids skipped)", bulk.Classified)
	}
	c.do("GET", g+"/materials?inbox=true", owner.AccessToken, nil, 200, &waiting)
	if len(waiting.Items) != 0 || waiting.InboxCount == nil || *waiting.InboxCount != 0 {
		t.Fatalf("inbox after bulk = %+v", waiting)
	}
	c.do("GET", "/materials/"+bulkIDs[1], owner.AccessToken, nil, 200, &details)
	if details.SubjectID == nil || *details.SubjectID != matan.ID || details.Kind != "NOTES" || details.Classification.Method != "manual" {
		t.Fatalf("bulk result = %+v", details)
	}
	c.do("GET", g+"/activity?limit=100", owner.AccessToken, nil, 200, &activity)
	kinds = map[string]int{}
	for _, a := range activity.Items {
		kinds[a.Kind]++
	}
	if kinds["material.bulk_classified"] != 1 {
		t.Fatalf("bulk action must be summarised once: %v", kinds)
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

	// --- publishing an upload to Drive (D32, D34) ---
	c.do("POST", g+"/materials/uploads", student.AccessToken, map[string]any{"file_name": "a.pdf", "size_bytes": 10, "to_drive": true}, 403, nil)
	c.do("POST", "/me/credentials", student.AccessToken, map[string]any{"email": "student-" + runID + "@example.com", "password": "correct horse"}, 200, nil)
	e.queue.Drain()
	// The folder is in "My Drive": the service account has no quota there, so
	// uploads wait for the head's Google account.
	problem.Type = ""
	c.do("POST", g+"/materials/uploads", student.AccessToken, map[string]any{"file_name": "a.pdf", "size_bytes": 10, "to_drive": true}, 503, &problem)
	if problem.Type != domain.ErrorPrefix+domain.CodeDrivePublisherRequired {
		t.Fatalf("to_drive without a publisher: type %q", problem.Type)
	}
	connectDrivePublisher(t, e, g, owner.AccessToken, student.AccessToken)

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
	d.QuotaExceeded = true // the service account could not do it: the head's account does
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

	// A member's upload is published later by the headman.
	laterUp := upload(student.AccessToken, "later.pdf", []byte("%PDF-1.4 later"), nil)
	var later material
	c.do("POST", "/uploads/"+laterUp.UploadID+"/complete", student.AccessToken, nil, 200, &later)
	e.queue.Drain()
	c.do("POST", "/materials/"+later.ID+"/publish-drive", student.AccessToken, nil, 403, nil)
	c.do("POST", "/materials/"+later.ID+"/publish-drive", owner.AccessToken, nil, 200, &later)
	if later.File.DriveUploadStatus == nil || *later.File.DriveUploadStatus != "PENDING" {
		t.Fatalf("publish later = %+v", later.File)
	}
	if jobs = e.queue.Drain(); len(jobs) != 1 || jobs[0].Type != domain.JobDriveUpload {
		t.Fatalf("jobs after publishing later = %+v", jobs)
	}
	if err := e.svc.Drive.UploadVersion(ctx, uuid.MustParse(later.ID), uuid.MustParse(later.File.ID)); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.FindByName("later.pdf"); !ok {
		t.Fatal("the later file was not created on Drive")
	}
	c.do("POST", "/materials/"+later.ID+"/publish-drive", owner.AccessToken, nil, 409, nil)

	// Access revoked at Google: the upload fails with a hint, the account is
	// flagged and new uploads to Drive are refused until it is reconnected.
	if err := e.oauth.Revoke(ctx, "refresh-first-code"); err != nil {
		t.Fatal(err)
	}
	failing := upload(student.AccessToken, "second.pdf", []byte("%PDF-1.4 second"), map[string]any{"to_drive": true})
	var second material
	c.do("POST", "/uploads/"+failing.UploadID+"/complete", student.AccessToken, nil, 200, &second)
	if err := e.svc.Drive.UploadVersion(ctx, uuid.MustParse(second.ID), uuid.MustParse(second.File.ID)); err == nil {
		t.Fatal("expected a permanent failure")
	}
	c.do("GET", "/materials/"+second.ID, student.AccessToken, nil, 200, &details)
	if details.File.DriveUploadStatus == nil || *details.File.DriveUploadStatus != "FAILED" || details.File.DriveUploadError == nil || !strings.Contains(*details.File.DriveUploadError, "reconnect") {
		t.Fatalf("revoked publisher = %+v", details.File)
	}
	var st driveStatus
	c.do("GET", g+"/drive/status", owner.AccessToken, nil, 200, &st)
	if st.CanPublish || st.Publisher == nil || st.Publisher.LastError == nil {
		t.Fatalf("status after revocation = %+v", st)
	}
	c.do("POST", g+"/materials/uploads", student.AccessToken, map[string]any{"file_name": "a.pdf", "size_bytes": 10, "to_drive": true}, 503, &problem)
	if problem.Type != domain.ErrorPrefix+domain.CodeDrivePublisherRevoked {
		t.Fatalf("to_drive with a revoked publisher: type %q", problem.Type)
	}
	d.QuotaExceeded = false

	// Disconnecting forgets the account and revokes its token at Google.
	c.do("DELETE", g+"/drive/publisher", student.AccessToken, nil, 403, nil)
	c.do("DELETE", g+"/drive/publisher", owner.AccessToken, nil, 204, nil)
	c.do("DELETE", g+"/drive/publisher", owner.AccessToken, nil, 404, nil)
	c.do("GET", g+"/drive/status", owner.AccessToken, nil, 200, &st)
	if st.Publisher != nil || st.CanPublish {
		t.Fatalf("status after disconnect = %+v", st)
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

	// --- file type filter (D35): Drive files of any type are indexed ---
	d.AddFile(misc, "Лекция 3 (запись).mp3", "audio/mpeg", "ID3 audio")
	d.AddFile(misc, "photo.jpg", "image/jpeg", "\xff\xd8\xff photo")
	d.AddFile(misc, "data.bin", "application/octet-stream", "raw")
	if _, err := e.svc.Drive.Sync(ctx, connID, false); err != nil {
		t.Fatal(err)
	}
	byType := func(ft string) []material {
		t.Helper()
		var p page
		c.do("GET", g+"/materials?limit=100&file_type="+ft, student.AccessToken, nil, 200, &p)
		return p.Items
	}
	if a := byType("AUDIO"); len(a) != 1 || a[0].File.Mime != "audio/mpeg" {
		t.Fatalf("audio filter = %+v", a)
	}
	if im := byType("IMAGE"); len(im) != 1 {
		t.Fatalf("image filter = %+v", im)
	}
	if o := byType("OTHER"); len(o) != 1 || o[0].File.Mime != "application/octet-stream" {
		t.Fatalf("other filter = %+v", o)
	}
	for _, doc := range byType("DOCUMENT") {
		if strings.HasPrefix(doc.File.Mime, "audio/") || strings.HasPrefix(doc.File.Mime, "image/") {
			t.Fatalf("document filter leaked %s", doc.File.Mime)
		}
	}
	if len(byType("DOCUMENT")) == 0 || len(byType("VIDEO")) != 0 {
		t.Fatal("document/video filter")
	}
	c.do("GET", g+"/materials?file_type=SPREADSHEET", student.AccessToken, nil, 422, nil)

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
	// --- PDF previews of office files ---
	docx := []byte("PK\x03\x04 lecture slides")
	officeTicket := upload(student.AccessToken, "Лекция 5.docx", docx, nil)
	var office material
	c.do("POST", "/uploads/"+officeTicket.UploadID+"/complete", student.AccessToken, nil, 200, &office)
	if office.File.PreviewStatus != "PENDING" {
		t.Fatalf("an uploaded office file is converted right away: %+v", office.File)
	}
	jobs = e.queue.Drain()
	if len(jobs) != 2 || jobs[1].Type != domain.JobMaterialPreview {
		t.Fatalf("jobs after an office upload = %+v", jobs)
	}
	c.do("POST", "/materials/"+office.ID+"/preview", student.AccessToken, nil, 202, nil)
	if jobs := e.queue.Drain(); len(jobs) != 0 {
		t.Fatalf("a pending preview must not be enqueued twice: %+v", jobs)
	}
	var officeLinks openLinks
	c.do("GET", "/materials/"+office.ID+"/open", student.AccessToken, nil, 200, &officeLinks)
	if officeLinks.PreviewURL != nil || officeLinks.PreviewStatus != "PENDING" || officeLinks.DownloadURL == nil {
		t.Fatalf("open before conversion = %+v", officeLinks)
	}
	if err := e.svc.Materials.BuildPreview(ctx, uuid.MustParse(office.File.ID)); err != nil {
		t.Fatal(err)
	}
	c.do("GET", "/materials/"+office.ID+"/open", student.AccessToken, nil, 200, &officeLinks)
	if officeLinks.PreviewURL == nil || officeLinks.PreviewStatus != "READY" {
		t.Fatalf("open after conversion = %+v", officeLinks)
	}
	code, hdr, body = c.raw("GET", *officeLinks.PreviewURL, nil, nil)
	if code != 200 || string(body) != "%PDF-1.7 preview of .docx: PK\x03\x04 lecture slides" ||
		hdr.Get("Content-Type") != "application/pdf" || !strings.HasPrefix(hdr.Get("Content-Disposition"), "inline") ||
		!strings.Contains(hdr.Get("Content-Disposition"), ".pdf") {
		t.Fatalf("preview: %d %v %q", code, hdr, body)
	}
	c.do("GET", "/materials/"+office.ID+"/open?download=true", student.AccessToken, nil, 200, &officeLinks)
	if officeLinks.PreviewURL != nil {
		t.Fatal("a download must not hand out the preview")
	}
	// Files shown as they are have no preview state.
	c.do("GET", "/materials/"+published.ID, student.AccessToken, nil, 200, &details)
	if details.File.PreviewStatus != "" {
		t.Fatalf("pdf preview state = %q", details.File.PreviewStatus)
	}
	c.do("POST", "/materials/"+published.ID+"/preview", student.AccessToken, nil, 422, nil)

	// Office files from Drive are converted on request, from Drive's bytes.
	d.AddFile(misc, "Слайды.pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", "PK\x03\x04 from drive")
	d.AddFile(misc, "Сломанный.pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", "broken")
	if _, err := e.svc.Drive.Sync(ctx, connID, false); err != nil {
		t.Fatal(err)
	}
	c.do("GET", g+"/materials?limit=100&q=pptx", owner.AccessToken, nil, 200, &filtered)
	for _, m := range filtered.Items {
		if m.File.PreviewStatus != "NONE" {
			t.Fatalf("drive office file before a request: %+v", m.File)
		}
		var st struct {
			PreviewStatus string `json:"preview_status"`
		}
		c.do("POST", "/materials/"+m.ID+"/preview", student.AccessToken, nil, 202, &st)
		if st.PreviewStatus != "PENDING" {
			t.Fatalf("preview request = %+v", st)
		}
		if err := e.svc.Materials.BuildPreview(ctx, uuid.MustParse(m.File.ID)); err != nil {
			t.Fatal(err)
		}
		c.do("GET", "/materials/"+m.ID, student.AccessToken, nil, 200, &details)
		want := "READY"
		if strings.Contains(m.Title, "Сломанный") {
			want = "FAILED"
		}
		if details.File.PreviewStatus != want {
			t.Fatalf("%s: preview %q, want %q", m.Title, details.File.PreviewStatus, want)
		}
		if want == "FAILED" {
			// A failed preview may be requested again.
			c.do("POST", "/materials/"+m.ID+"/preview", student.AccessToken, nil, 202, &st)
			if st.PreviewStatus != "PENDING" {
				t.Fatalf("retry after failure = %+v", st)
			}
		}
	}
	if len(filtered.Items) != 2 {
		t.Fatalf("drive office files = %d", len(filtered.Items))
	}
	// Two requests and a retry after the failure.
	if jobs := e.queue.Drain(); len(jobs) != 3 || jobs[2].Type != domain.JobMaterialPreview {
		t.Fatalf("preview jobs = %+v", jobs)
	}

	// A sync failure keeps a machine code for a translated message (managers only).
	d.Trash(root)
	if _, err := e.svc.Drive.Sync(ctx, connID, true); err == nil {
		t.Fatal("sync of a trashed folder must fail")
	}
	c.do("GET", g+"/drive/status", owner.AccessToken, nil, 200, &status)
	if status.Connection.Status != "ERROR" || status.Connection.LastErrorCode == nil || *status.Connection.LastErrorCode != domain.CodeFolderTrashed {
		t.Fatalf("failed sync status = %+v", status.Connection)
	}
	c.do("GET", g+"/drive/status", student.AccessToken, nil, 200, &status)
	if status.Connection.LastError != nil || status.Connection.LastErrorCode != nil {
		t.Fatalf("a student sees the sync error: %+v", status.Connection)
	}

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

// checkOldDriveVersion covers older versions of a Drive file: they are served
// from their revision (not the current content), versions indexed before
// revisions were recorded are matched by md5, and a revision Google no longer
// keeps is reported as version_unavailable.
func checkOldDriveVersion(t *testing.T, e *env, materialID, oldVersionID, fileID, token string) {
	t.Helper()
	c := e.c
	stream := func(versionID string, headers map[string]string) (int, http.Header, string) {
		t.Helper()
		var links openLinks
		c.do("GET", "/materials/"+materialID+"/open?version_id="+versionID, token, nil, 200, &links)
		if links.StreamURL == nil {
			t.Fatalf("open version %s = %+v", versionID, links)
		}
		if versionID == oldVersionID && links.DriveWebViewLink != nil {
			t.Fatal("an older version must not link to the current Drive file")
		}
		code, hdr, body := c.raw("GET", *links.StreamURL, nil, headers)
		return code, hdr, string(body)
	}
	if code, _, body := stream(oldVersionID, nil); code != 200 || body != "%PDF-1.4 lecture one" {
		t.Fatalf("old version: %d %q", code, body)
	}
	if code, hdr, body := stream(oldVersionID, map[string]string{"Range": "bytes=9-15"}); code != 206 || body != "lecture" || hdr.Get("Content-Range") != "bytes 9-15/20" {
		t.Fatalf("old version range: %d %q %v", code, body, hdr)
	}

	// Versions indexed before drive_revision_id existed: matched by md5.
	ctx := context.Background()
	if _, err := e.svc.Pool.Exec(ctx, `UPDATE material_versions SET drive_revision_id = NULL WHERE material_id = $1`, materialID); err != nil {
		t.Fatal(err)
	}
	if code, _, body := stream(oldVersionID, nil); code != 200 || body != "%PDF-1.4 lecture one" {
		t.Fatalf("legacy old version: %d %q", code, body)
	}
	var remembered *string
	if err := e.svc.Pool.QueryRow(ctx, `SELECT drive_revision_id FROM material_versions WHERE id = $1`, oldVersionID).Scan(&remembered); err != nil {
		t.Fatal(err)
	}
	if remembered == nil || *remembered == "" {
		t.Fatal("matched revision was not remembered")
	}

	// Google dropped the revision: a clear error instead of the current file.
	e.drive.DropOldRevisions(fileID)
	var problem struct {
		Type string `json:"type"`
	}
	c.do("GET", "/materials/"+materialID+"/open?version_id="+oldVersionID, token, nil, 410, &problem)
	if problem.Type != domain.ErrorPrefix+domain.CodeVersionUnavailable {
		t.Fatalf("dropped revision: type %q", problem.Type)
	}
}

type driveStatus struct {
	PublisherAvailable bool `json:"publisher_available"`
	CanPublish         bool `json:"can_publish"`
	Publisher          *struct {
		Email     string  `json:"email"`
		LastError *string `json:"last_error"`
	} `json:"publisher"`
}

// connectDrivePublisher runs the OAuth sign-in of the publishing account
// through the API: start → Google (fake) → callback page.
func connectDrivePublisher(t *testing.T, e *env, g, ownerToken, studentToken string) {
	t.Helper()
	c := e.c
	var st driveStatus
	c.do("GET", g+"/drive/status", ownerToken, nil, 200, &st)
	if !st.PublisherAvailable || st.CanPublish || st.Publisher != nil {
		t.Fatalf("status before connecting = %+v", st)
	}
	c.do("POST", g+"/drive/publisher", studentToken, nil, 403, nil)
	var start struct {
		AuthURL string `json:"auth_url"`
	}
	c.do("POST", g+"/drive/publisher", ownerToken, nil, 200, &start)
	u, err := url.Parse(start.AuthURL)
	if err != nil {
		t.Fatal(err)
	}
	state := u.Query().Get("state")
	callback := func(query string) (int, string) {
		t.Helper()
		code, hdr, body := c.raw("GET", c.base+"/drive/oauth/callback?"+query, nil, nil)
		if !strings.HasPrefix(hdr.Get("Content-Type"), "text/html") || hdr.Get("Cache-Control") != "no-store" {
			t.Fatalf("callback headers = %v", hdr)
		}
		return code, string(body)
	}
	if code, body := callback("error=access_denied&state=" + url.QueryEscape(state)); code != 400 || !strings.Contains(body, "отменён") {
		t.Fatalf("cancelled: %d %s", code, body)
	}
	if code, body := callback("code=denied-drive&state=" + url.QueryEscape(state)); code != 400 || !strings.Contains(body, "доступ к Диску") {
		t.Fatalf("drive scope unticked: %d %s", code, body)
	}
	if code, _ := callback("code=first-code&state=" + url.QueryEscape(state+"x")); code != 400 {
		t.Fatalf("tampered state: %d", code)
	}
	code, body := callback("code=first-code&state=" + url.QueryEscape(state))
	if code != 200 || !strings.Contains(body, "publisher@example.com") {
		t.Fatalf("connected: %d %s", code, body)
	}

	c.do("GET", g+"/drive/status", ownerToken, nil, 200, &st)
	if !st.CanPublish || st.Publisher == nil || st.Publisher.Email != "publisher@example.com" {
		t.Fatalf("status after connecting = %+v", st)
	}
	c.do("GET", g+"/drive/status", studentToken, nil, 200, &st)
	if !st.CanPublish || st.Publisher != nil {
		t.Fatalf("a student sees = %+v", st)
	}
	// The refresh token is stored sealed.
	var sealed []byte
	if err := e.svc.Pool.QueryRow(context.Background(),
		`SELECT refresh_token_enc FROM drive_publishers p JOIN groups gr ON gr.id = p.group_id WHERE $1 LIKE '%' || gr.id::text || '%'`, g).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if len(sealed) == 0 || bytes.Contains(sealed, []byte("refresh-first-code")) {
		t.Fatal("refresh token is stored in clear text")
	}
}
