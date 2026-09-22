package http

import (
	"context"
	nethttp "net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"heatseeker/api/internal/app/materials"
	"heatseeker/api/internal/domain"
)

type listMaterialsInput struct {
	GroupID   string   `path:"groupId" format:"uuid"`
	SubjectID string   `query:"subject_id" format:"uuid" doc:"Только материалы предмета."`
	NoSubject bool     `query:"no_subject" doc:"Только материалы без предмета."`
	TagIDs    []string `query:"tag_id" doc:"Материалы со всеми указанными тегами (тег предмета = фильтр по предмету)."`
	Kind      string   `query:"kind" enum:"LECTURE,NOTES,REPORT,CALC,ASSIGNMENT,OTHER"`
	FileType  string   `query:"file_type" enum:"DOCUMENT,IMAGE,AUDIO,VIDEO,ARCHIVE,OTHER" doc:"Тип файла по MIME (D35)."`
	Mine      bool     `query:"mine" doc:"Только загруженные мной."`
	Inbox     bool     `query:"inbox" doc:"«Входящие»: материалы, требующие разбора."`
	Archived  bool     `query:"archived" doc:"Архив вместо актуальных."`
	Query     string   `query:"q" maxLength:"200" doc:"Поиск по названию и имени файла."`
	Cursor    string   `query:"cursor"`
	Limit     int32    `query:"limit" minimum:"1" maximum:"100" default:"30"`
}

type materialsPageOutput struct {
	Body struct {
		Items      []MaterialDTO `json:"items"`
		NextCursor string        `json:"next_cursor,omitempty"`
		InboxCount *int64        `json:"inbox_count,omitempty" doc:"Размер «Входящих» (для модераторов и при inbox=true)."`
	}
}

type materialIDInput struct {
	MaterialID string `path:"materialId" format:"uuid"`
}

type materialOutput struct {
	Body MaterialDTO
}

// ClassifiedDTO is a material decided in the Inbox.
type ClassifiedDTO struct {
	MaterialDTO
	LearnedAlias string `json:"learned_alias,omitempty" doc:"Синоним, добавленный предмету из имени папки на Диске; похожие файлы разберутся автоматически."`
}

type classifiedOutput struct {
	Body ClassifiedDTO
}

type materialDetailsOutput struct {
	Body MaterialDetailsDTO
}

type updateMaterialInput struct {
	MaterialID string `path:"materialId" format:"uuid"`
	Body       struct {
		Title        *string   `json:"title,omitempty" minLength:"1" maxLength:"200"`
		Description  *string   `json:"description,omitempty" maxLength:"10000"`
		SubjectID    *string   `json:"subject_id,omitempty" format:"uuid"`
		ClearSubject bool      `json:"clear_subject,omitempty" doc:"Убрать предмет."`
		Kind         *string   `json:"kind,omitempty" enum:"LECTURE,NOTES,REPORT,CALC,ASSIGNMENT,OTHER"`
		TagIDs       *[]string `json:"tag_ids,omitempty" maxItems:"20"`
	}
}

type classifyMaterialInput struct {
	MaterialID string `path:"materialId" format:"uuid"`
	Body       struct {
		SubjectID  *string `json:"subject_id,omitempty" format:"uuid" doc:"Пусто — «без предмета»."`
		Kind       string  `json:"kind,omitempty" enum:"LECTURE,NOTES,REPORT,CALC,ASSIGNMENT,OTHER"`
		LearnAlias bool    `json:"learn_alias,omitempty" doc:"Запомнить папку на Диске как синоним предмета."`
	}
}

type bulkClassifyInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    struct {
		MaterialIDs []string `json:"material_ids" minItems:"1" maxItems:"200"`
		SubjectID   *string  `json:"subject_id,omitempty" format:"uuid" doc:"Пусто — «без предмета»."`
		Kind        *string  `json:"kind,omitempty" enum:"LECTURE,NOTES,REPORT,CALC,ASSIGNMENT,OTHER" doc:"Пусто — тип каждого материала не меняется."`
	}
}

type bulkClassifyOutput struct {
	Body struct {
		Classified int `json:"classified" doc:"Сколько материалов разобрано (чужие и удалённые пропускаются)."`
	}
}

type openMaterialInput struct {
	MaterialID string `path:"materialId" format:"uuid"`
	VersionID  string `query:"version_id" format:"uuid"`
	Download   bool   `query:"download" doc:"Скачать вместо просмотра."`
}

type openOutput struct {
	Body OpenDTO
}

type materialPreviewInput struct {
	MaterialID string `path:"materialId" format:"uuid"`
	VersionID  string `query:"version_id" format:"uuid" doc:"Версия; по умолчанию текущая."`
}

type materialPreviewOutput struct {
	Body struct {
		PreviewStatus string `json:"preview_status" enum:"PENDING,READY,SKIPPED" doc:"READY — превью в preview_url ответа /open; PENDING — готовится, опрашивайте материал."`
	}
}

type createUploadInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    struct {
		FileName    string   `json:"file_name" minLength:"1" maxLength:"255"`
		SizeBytes   int64    `json:"size_bytes" minimum:"1"`
		Mime        string   `json:"mime,omitempty" maxLength:"255"`
		Title       string   `json:"title,omitempty" maxLength:"200" doc:"По умолчанию — из имени файла."`
		Description string   `json:"description,omitempty" maxLength:"10000"`
		SubjectID   *string  `json:"subject_id,omitempty" format:"uuid" doc:"Не указан — угадывается по имени файла."`
		Kind        *string  `json:"kind,omitempty" enum:"LECTURE,NOTES,REPORT,CALC,ASSIGNMENT,OTHER"`
		TagIDs      []string `json:"tag_ids,omitempty" maxItems:"20"`
		ToDrive     bool     `json:"to_drive,omitempty" doc:"Опубликовать копию в папке группы на Google Диске (нужен защищённый аккаунт)."`
		TaskID      *string  `json:"task_id,omitempty" format:"uuid" doc:"Сразу прикрепить к задаче (нужно право править задачу)."`
		TaskOnly    bool     `json:"task_only,omitempty" doc:"Оставить файл только в задаче: не в ленте, видят те, кто видит задачу (D43). Нужен task_id; не вместе с to_drive."`
	}
}

type uploadTicketOutput struct {
	Body UploadTicketDTO
}

type uploadIDInput struct {
	UploadID string `path:"uploadId" format:"uuid"`
}

// UploadLimitsDTO describes what may be uploaded.
type UploadLimitsDTO struct {
	MaxBytes   int64    `json:"max_bytes"`
	AllowedExt []string `json:"allowed_ext"`
}

// FeaturesDTO lists server features the client adapts to.
type FeaturesDTO struct {
	DriveUpload bool `json:"drive_upload" doc:"Можно публиковать загрузки на Google Диск."`
	DriveProxy  bool `json:"drive_proxy" doc:"Файлы с Диска можно смотреть через сервер."`
}

type metaOutput struct {
	Body struct {
		Upload   UploadLimitsDTO `json:"upload"`
		Features FeaturesDTO     `json:"features"`
		Kinds    []string        `json:"material_kinds"`
	}
}

func registerMaterials(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "meta", Method: nethttp.MethodGet, Path: "/meta", Tags: []string{"meta"},
		Summary: "Ограничения и возможности сервера",
	}, func(ctx context.Context, _ *struct{}) (*metaOutput, error) {
		out := &metaOutput{}
		if d.Materials != nil {
			l := d.Materials.Limits()
			out.Body.Upload.MaxBytes = l.MaxUploadBytes
			out.Body.Upload.AllowedExt = l.AllowedExt
			out.Body.Features.DriveUpload = l.DriveUploadEnabled
			out.Body.Features.DriveProxy = l.ProxyEnabled
		}
		for _, k := range domain.AllMaterialKinds {
			out.Body.Kinds = append(out.Body.Kinds, string(k))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "materials-list", Method: nethttp.MethodGet, Path: "/groups/{groupId}/materials", Tags: []string{"materials"}, Security: bearer,
		Summary: "Лента материалов", Description: "Фильтры комбинируются. Пагинация курсором: передайте next_cursor в cursor.",
	}, func(ctx context.Context, in *listMaterialsInput) (*materialsPageOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		li := materials.ListInput{
			NoSubject: in.NoSubject, Mine: in.Mine, Inbox: in.Inbox, Archived: in.Archived,
			Query: in.Query, Cursor: in.Cursor, Limit: in.Limit,
		}
		if li.SubjectID, err = parseOptionalID("subject_id", in.SubjectID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		if li.TagIDs, err = parseIDs("tag_id", in.TagIDs); err != nil {
			return nil, apiErr(d.Log, err)
		}
		if in.Kind != "" {
			k := domain.MaterialKind(in.Kind)
			li.Kind = &k
		}
		if in.FileType != "" {
			ft := domain.FileType(in.FileType)
			li.FileType = &ft
		}
		page, err := d.Materials.List(ctx, p.UserID, groupID, li)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &materialsPageOutput{}
		out.Body.Items = make([]MaterialDTO, len(page.Items))
		for i := range page.Items {
			out.Body.Items[i] = toMaterialDTO(&page.Items[i], d.Materials)
		}
		out.Body.NextCursor = page.NextCursor
		if page.InboxCount > 0 || in.Inbox {
			n := page.InboxCount
			out.Body.InboxCount = &n
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "materials-get", Method: nethttp.MethodGet, Path: "/materials/{materialId}", Tags: []string{"materials"}, Security: bearer,
		Summary: "Материал с историей версий",
	}, func(ctx context.Context, in *materialIDInput) (*materialDetailsOutput, error) {
		p, id, err := principalAndID(ctx, "materialId", in.MaterialID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		details, err := d.Materials.Get(ctx, p.UserID, id)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &materialDetailsOutput{Body: toMaterialDetailsDTO(details, d.Materials)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "materials-update", Method: nethttp.MethodPatch, Path: "/materials/{materialId}", Tags: []string{"materials"}, Security: bearer,
		Summary: "Изменить материал", Description: "Своё — загрузившему, любое — модератору. Смена предмета или типа считается ручной классификацией.",
	}, func(ctx context.Context, in *updateMaterialInput) (*materialOutput, error) {
		p, id, err := principalAndID(ctx, "materialId", in.MaterialID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		b := in.Body
		ui := materials.UpdateInput{Title: b.Title, Description: b.Description}
		switch {
		case b.ClearSubject:
			ui.SetSubject = true
		case b.SubjectID != nil:
			ui.SetSubject = true
			if ui.SubjectID, err = parseOptionalID("subject_id", *b.SubjectID); err != nil {
				return nil, apiErr(d.Log, err)
			}
		}
		if b.Kind != nil {
			k := domain.MaterialKind(*b.Kind)
			ui.Kind = &k
		}
		if b.TagIDs != nil {
			tags, err := parseIDs("tag_ids", *b.TagIDs)
			if err != nil {
				return nil, apiErr(d.Log, err)
			}
			ui.TagIDs = &tags
		}
		view, err := d.Materials.Update(ctx, p.UserID, id, ui)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &materialOutput{Body: toMaterialDTO(view, d.Materials)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "materials-classify-bulk", Method: nethttp.MethodPost, Path: "/groups/{groupId}/materials/classify", Tags: []string{"materials"}, Security: bearer,
		Summary: "Разобрать несколько материалов", Description: "Массовое действие во «Входящих»: один предмет (и, при желании, тип) для всех выбранных.",
	}, func(ctx context.Context, in *bulkClassifyInput) (*bulkClassifyOutput, error) {
		p, groupID, err := principalAndID(ctx, "groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		ids, err := parseIDs("material_ids", in.Body.MaterialIDs)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		bi := materials.BulkClassifyInput{MaterialIDs: ids}
		if in.Body.SubjectID != nil {
			if bi.SubjectID, err = parseOptionalID("subject_id", *in.Body.SubjectID); err != nil {
				return nil, apiErr(d.Log, err)
			}
		}
		if in.Body.Kind != nil {
			k := domain.MaterialKind(*in.Body.Kind)
			bi.Kind = &k
		}
		n, err := d.Materials.ClassifyMany(ctx, p.UserID, groupID, bi)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &bulkClassifyOutput{}
		out.Body.Classified = n
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "materials-classify", Method: nethttp.MethodPost, Path: "/materials/{materialId}/classify", Tags: []string{"materials"}, Security: bearer,
		Summary: "Разобрать материал из «Входящих»", Description: "Модератор подтверждает предмет и тип; материал уходит из «Входящих».",
	}, func(ctx context.Context, in *classifyMaterialInput) (*classifiedOutput, error) {
		p, id, err := principalAndID(ctx, "materialId", in.MaterialID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		ci := materials.ClassifyInput{Kind: domain.MaterialKind(in.Body.Kind), LearnAlias: in.Body.LearnAlias}
		if in.Body.SubjectID != nil {
			if ci.SubjectID, err = parseOptionalID("subject_id", *in.Body.SubjectID); err != nil {
				return nil, apiErr(d.Log, err)
			}
		}
		res, err := d.Materials.Classify(ctx, p.UserID, id, ci)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &classifiedOutput{Body: ClassifiedDTO{MaterialDTO: toMaterialDTO(res.View, d.Materials), LearnedAlias: res.LearnedAlias}}, nil
	})

	transition := func(opID, path, summary, description string, method string, fn func(context.Context, uuid.UUID, uuid.UUID) error) {
		huma.Register(api, huma.Operation{
			OperationID: opID, Method: method, Path: path, Tags: []string{"materials"}, Security: bearer,
			Summary: summary, Description: description, DefaultStatus: nethttp.StatusNoContent,
		}, func(ctx context.Context, in *materialIDInput) (*emptyOutput, error) {
			p, id, err := principalAndID(ctx, "materialId", in.MaterialID)
			if err != nil {
				return nil, apiErr(d.Log, err)
			}
			if err := fn(ctx, p.UserID, id); err != nil {
				return nil, apiErr(d.Log, err)
			}
			return &emptyOutput{}, nil
		})
	}
	transition("materials-archive", "/materials/{materialId}/archive", "Архивировать материал",
		"Своё — загрузившему, любое — модератору.", nethttp.MethodPost, d.Materials.Archive)
	transition("materials-restore", "/materials/{materialId}/restore", "Вернуть материал из архива",
		"Удалённый (до окончательной очистки) возвращает только модератор.", nethttp.MethodPost, d.Materials.Restore)
	transition("materials-delete", "/materials/{materialId}", "Удалить материал",
		"Своё — пока его никто не открывал; любое — модератору. Файлы на Google Диске не удаляются.", nethttp.MethodDelete, d.Materials.Delete)

	huma.Register(api, huma.Operation{
		OperationID: "materials-open", Method: nethttp.MethodGet, Path: "/materials/{materialId}/open", Tags: []string{"materials"}, Security: bearer,
		Summary: "Ссылки для просмотра и скачивания", Description: "Ссылки временные (см. expires_at) и не требуют заголовка Authorization.",
	}, func(ctx context.Context, in *openMaterialInput) (*openOutput, error) {
		p, id, err := principalAndID(ctx, "materialId", in.MaterialID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		oi := materials.OpenInput{Download: in.Download}
		if oi.VersionID, err = parseOptionalID("version_id", in.VersionID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		res, err := d.Materials.Open(ctx, p.UserID, id, oi)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &openOutput{Body: toOpenDTO(res)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "materials-share", Method: nethttp.MethodPost, Path: "/materials/{materialId}/share", Tags: []string{"materials"}, Security: bearer,
		Summary:     "Открыть файл из задачи группе",
		Description: "Файл, оставленный только в задаче, становится обычным материалом: появляется в ленте и остаётся прикреплённым к задаче. Загрузивший, автор задачи или модератор.",
	}, func(ctx context.Context, in *materialIDInput) (*materialOutput, error) {
		p, id, err := principalAndID(ctx, "materialId", in.MaterialID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		view, err := d.Materials.Share(ctx, p.UserID, id)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &materialOutput{Body: toMaterialDTO(view, d.Materials)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "materials-publish-drive", Method: nethttp.MethodPost, Path: "/materials/{materialId}/publish-drive", Tags: []string{"materials"}, Security: bearer,
		Summary:     "Опубликовать загруженный файл на Google Диск",
		Description: "Копия файла из хранилища приложения уходит в папку группы на Диске (как загрузка с галочкой «на Диск»); после неудачи — пробует снова. Староста, модератор или админ с защищённым аккаунтом.",
	}, func(ctx context.Context, in *materialIDInput) (*materialOutput, error) {
		p, id, err := principalAndID(ctx, "materialId", in.MaterialID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		view, err := d.Materials.PublishToDrive(ctx, p.UserID, id)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &materialOutput{Body: toMaterialDTO(view, d.Materials)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "materials-preview", Method: nethttp.MethodPost, Path: "/materials/{materialId}/preview", Tags: []string{"materials"}, Security: bearer,
		Summary: "Запросить PDF-превью офисного файла",
		Description: "pptx/docx/xlsx и т.п. браузер не показывает: сервер конвертирует их в PDF (Gotenberg). " +
			"Повторный запрос не создаёт новую задачу; после неудачи (FAILED) — пробует снова.",
		DefaultStatus: nethttp.StatusAccepted,
	}, func(ctx context.Context, in *materialPreviewInput) (*materialPreviewOutput, error) {
		p, id, err := principalAndID(ctx, "materialId", in.MaterialID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		versionID, err := parseOptionalID("version_id", in.VersionID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		status, err := d.Materials.RequestPreview(ctx, p.UserID, id, versionID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &materialPreviewOutput{}
		out.Body.PreviewStatus = status
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "uploads-create", Method: nethttp.MethodPost, Path: "/groups/{groupId}/materials/uploads", Tags: []string{"materials"}, Security: bearer,
		Summary:       "Начать загрузку файла",
		Description:   "Возвращает адрес, куда отправить файл методом PUT с указанными заголовками. После загрузки вызовите /uploads/{uploadId}/complete.",
		DefaultStatus: nethttp.StatusCreated,
	}, func(ctx context.Context, in *createUploadInput) (*uploadTicketOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		b := in.Body
		ui := materials.UploadInput{
			FileName: b.FileName, SizeBytes: b.SizeBytes, Mime: b.Mime, Title: b.Title,
			Description: b.Description, ToDrive: b.ToDrive, TaskOnly: b.TaskOnly,
		}
		if b.TaskID != nil {
			if ui.TaskID, err = parseOptionalID("task_id", *b.TaskID); err != nil {
				return nil, apiErr(d.Log, err)
			}
		}
		if b.SubjectID != nil {
			if ui.SubjectID, err = parseOptionalID("subject_id", *b.SubjectID); err != nil {
				return nil, apiErr(d.Log, err)
			}
		}
		if b.Kind != nil {
			k := domain.MaterialKind(*b.Kind)
			ui.Kind = &k
		}
		if ui.TagIDs, err = parseIDs("tag_ids", b.TagIDs); err != nil {
			return nil, apiErr(d.Log, err)
		}
		ticket, err := d.Materials.CreateUpload(ctx, p.UserID, groupID, ui)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &uploadTicketOutput{Body: toUploadTicketDTO(ticket)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "uploads-complete", Method: nethttp.MethodPost, Path: "/uploads/{uploadId}/complete", Tags: []string{"materials"}, Security: bearer,
		Summary: "Завершить загрузку", Description: "Проверяет файл и создаёт материал. Повторный вызов вернёт тот же материал.",
	}, func(ctx context.Context, in *uploadIDInput) (*materialOutput, error) {
		p, id, err := principalAndID(ctx, "uploadId", in.UploadID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		view, err := d.Materials.CompleteUpload(ctx, p.UserID, id)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &materialOutput{Body: toMaterialDTO(view, d.Materials)}, nil
	})
}

func principalAndID(ctx context.Context, field, raw string) (Principal, uuid.UUID, error) {
	p, err := principalFrom(ctx)
	if err != nil {
		return Principal{}, uuid.Nil, err
	}
	id, err := parseID(field, raw)
	return p, id, err
}

func parseOptionalID(field, raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := parseID(field, raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func parseIDs(field string, raw []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(raw))
	for _, r := range raw {
		id, err := parseID(field, r)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}
