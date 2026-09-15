package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"

	"github.com/danielgtaylor/huma/v2"

	"heatseeker/api/internal/app/groups"
	"heatseeker/api/internal/domain"
)

type createGroupInput struct {
	Body struct {
		Name string `json:"name" minLength:"2" maxLength:"80"`
		Kind string `json:"kind,omitempty" enum:"MASTERS,DPO,OTHER" doc:"По умолчанию MASTERS. DPO — отдельный поток с выключенными по умолчанию уведомлениями."`
	}
}

type groupWithMembershipOutput struct {
	Body GroupWithMembershipDTO
}

type groupIDInput struct {
	GroupID string `path:"groupId" format:"uuid"`
}

type groupOutput struct {
	Body GroupDTO
}

type joinCodeInput struct {
	Code string `path:"code" minLength:"4" maxLength:"64"`
}

type previewOutput struct {
	Body struct {
		Group       GroupDTO `json:"group"`
		Roles       []string `json:"roles"`
		MemberCount int64    `json:"member_count"`
		ViaInvite   bool     `json:"via_invite"`
	}
}

type membersOutput struct {
	Body struct {
		Items    []MemberDTO `json:"items"`
		Extended bool        `json:"extended" doc:"true — ответ содержит расширенные поля пользователей (админ)."`
	}
}

type memberRolesInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	UserID  string `path:"userId" format:"uuid"`
	Body    struct {
		Roles []string `json:"roles" minItems:"1" doc:"Полный набор ролей участника."`
	}
}

type memberStatusInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	UserID  string `path:"userId" format:"uuid"`
	Body    struct {
		Status string `json:"status" enum:"ACTIVE,BANNED"`
	}
}

type membershipOutput struct {
	Body MembershipDTO
}

type createInviteInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    struct {
		Roles       []string `json:"roles,omitempty" doc:"Роли приглашённого; по умолчанию STUDENT."`
		ExpiresDays *int     `json:"expires_days,omitempty" minimum:"0" maximum:"365" doc:"0 — бессрочно."`
		MaxUses     *int32   `json:"max_uses,omitempty" minimum:"1"`
	}
}

type inviteOutput struct {
	Body InviteDTO
}

type invitesOutput struct {
	Body struct {
		Items []InviteDTO `json:"items"`
	}
}

type inviteIDInput struct {
	GroupID  string `path:"groupId" format:"uuid"`
	InviteID string `path:"inviteId" format:"uuid"`
}

type updateGroupInput struct {
	GroupID string `path:"groupId" format:"uuid"`
	Body    struct {
		Name       *string         `json:"name,omitempty" minLength:"2" maxLength:"80"`
		Kind       *string         `json:"kind,omitempty" enum:"MASTERS,DPO,OTHER"`
		JoinPolicy *string         `json:"join_policy,omitempty" enum:"OPEN,INVITE"`
		PublicRead *bool           `json:"public_read,omitempty"`
		MediaMode  *string         `json:"media_mode,omitempty" enum:"LINK,CACHE,IMPORT"`
		Settings   json.RawMessage `json:"settings,omitempty"`
	}
}

func registerGroups(api huma.API, d Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "groups-create", Method: nethttp.MethodPost, Path: "/groups", Tags: []string{"groups"}, Security: bearer,
		Summary: "Создать группу", DefaultStatus: nethttp.StatusCreated,
	}, func(ctx context.Context, in *createGroupInput) (*groupWithMembershipOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		gm, err := d.Groups.Create(ctx, p.UserID, groups.CreateInput{Name: in.Body.Name, Kind: domain.GroupKind(in.Body.Kind)})
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &groupWithMembershipOutput{Body: toGroupWithMembershipDTO(*gm)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "groups-preview", Method: nethttp.MethodGet, Path: "/groups/join/{code}", Tags: []string{"groups"},
		Summary: "Что за группа стоит за кодом", Description: "Без аутентификации — для экрана «Вступить в группу?».",
	}, func(ctx context.Context, in *joinCodeInput) (*previewOutput, error) {
		pv, err := d.Groups.Preview(ctx, in.Code)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &previewOutput{}
		g := toGroupDTO(&pv.Group)
		g.JoinCode = nil
		g.Settings = json.RawMessage("{}")
		out.Body.Group = g
		out.Body.Roles = domain.RolesToStrings(pv.Roles)
		out.Body.MemberCount = pv.MemberCount
		out.Body.ViaInvite = pv.ViaInvite
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "groups-join", Method: nethttp.MethodPost, Path: "/groups/join/{code}", Tags: []string{"groups"}, Security: bearer,
		Summary: "Вступить по коду приглашения или коду группы",
	}, func(ctx context.Context, in *joinCodeInput) (*groupWithMembershipOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		gm, err := d.Groups.Join(ctx, p.UserID, in.Code)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &groupWithMembershipOutput{Body: toGroupWithMembershipDTO(*gm)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "groups-get", Method: nethttp.MethodGet, Path: "/groups/{groupId}", Tags: []string{"groups"}, Security: bearer,
		Summary: "Группа и моё членство",
	}, func(ctx context.Context, in *groupIDInput) (*groupWithMembershipOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		actor, err := d.Groups.Get(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &groupWithMembershipOutput{Body: toGroupWithMembershipDTO(domain.GroupWithMembership{Group: *actor.Group, Membership: *actor.Membership})}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "groups-update", Method: nethttp.MethodPatch, Path: "/groups/{groupId}", Tags: []string{"groups"}, Security: bearer,
		Summary: "Настройки группы",
	}, func(ctx context.Context, in *updateGroupInput) (*groupOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		upd := groups.UpdateInput{Name: in.Body.Name, PublicRead: in.Body.PublicRead, Settings: in.Body.Settings}
		if in.Body.Kind != nil {
			k := domain.GroupKind(*in.Body.Kind)
			upd.Kind = &k
		}
		if in.Body.JoinPolicy != nil {
			jp := domain.JoinPolicy(*in.Body.JoinPolicy)
			upd.JoinPolicy = &jp
		}
		if in.Body.MediaMode != nil {
			mm := domain.MediaMode(*in.Body.MediaMode)
			upd.MediaMode = &mm
		}
		g, err := d.Groups.Update(ctx, p.UserID, groupID, upd)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &groupOutput{Body: toGroupDTO(g)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "groups-rotate-join-code", Method: nethttp.MethodPost, Path: "/groups/{groupId}/join-code/rotate", Tags: []string{"groups"}, Security: bearer,
		Summary: "Сменить код группы",
	}, func(ctx context.Context, in *groupIDInput) (*groupOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		g, err := d.Groups.RotateJoinCode(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &groupOutput{Body: toGroupDTO(g)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "groups-members", Method: nethttp.MethodGet, Path: "/groups/{groupId}/members", Tags: []string{"groups"}, Security: bearer,
		Summary: "Участники группы",
	}, func(ctx context.Context, in *groupIDInput) (*membersOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		res, err := d.Groups.Members(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &membersOutput{}
		out.Body.Extended = res.Extended
		out.Body.Items = make([]MemberDTO, len(res.Members))
		for i, m := range res.Members {
			out.Body.Items[i] = MemberDTO{User: toPublicUserDTO(&m.User, res.Extended), Membership: toMembershipDTO(&m.Membership, m.Membership.UserID == p.UserID)}
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "groups-member-roles", Method: nethttp.MethodPut, Path: "/groups/{groupId}/members/{userId}/roles", Tags: []string{"groups"}, Security: bearer,
		Summary: "Назначить роли участнику",
	}, func(ctx context.Context, in *memberRolesInput) (*membershipOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		userID, err := parseID("userId", in.UserID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		roles, err := parseRoles(in.Body.Roles)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		m, err := d.Groups.UpdateRoles(ctx, p.UserID, groupID, userID, roles)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &membershipOutput{Body: toMembershipDTO(m, false)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "groups-member-status", Method: nethttp.MethodPut, Path: "/groups/{groupId}/members/{userId}/status", Tags: []string{"groups"}, Security: bearer,
		Summary: "Заблокировать или восстановить участника",
	}, func(ctx context.Context, in *memberStatusInput) (*membershipOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		userID, err := parseID("userId", in.UserID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		m, err := d.Groups.SetStatus(ctx, p.UserID, groupID, userID, domain.MembershipStatus(in.Body.Status))
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &membershipOutput{Body: toMembershipDTO(m, false)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "invites-create", Method: nethttp.MethodPost, Path: "/groups/{groupId}/invites", Tags: []string{"groups"}, Security: bearer,
		Summary: "Создать приглашение", DefaultStatus: nethttp.StatusCreated,
	}, func(ctx context.Context, in *createInviteInput) (*inviteOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		roles, err := parseRoles(in.Body.Roles)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		inv, err := d.Groups.CreateInvite(ctx, p.UserID, groupID, groups.InviteInput{Roles: roles, ExpiresDays: in.Body.ExpiresDays, MaxUses: in.Body.MaxUses})
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &inviteOutput{Body: toInviteDTO(inv)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "invites-list", Method: nethttp.MethodGet, Path: "/groups/{groupId}/invites", Tags: []string{"groups"}, Security: bearer,
		Summary: "Активные приглашения",
	}, func(ctx context.Context, in *groupIDInput) (*invitesOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		items, err := d.Groups.ListInvites(ctx, p.UserID, groupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		out := &invitesOutput{}
		out.Body.Items = make([]InviteDTO, len(items))
		for i := range items {
			out.Body.Items[i] = toInviteDTO(&items[i])
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "invites-revoke", Method: nethttp.MethodDelete, Path: "/groups/{groupId}/invites/{inviteId}", Tags: []string{"groups"}, Security: bearer,
		Summary: "Отозвать приглашение", DefaultStatus: nethttp.StatusNoContent,
	}, func(ctx context.Context, in *inviteIDInput) (*emptyOutput, error) {
		p, err := principalFrom(ctx)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		groupID, err := parseID("groupId", in.GroupID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		inviteID, err := parseID("inviteId", in.InviteID)
		if err != nil {
			return nil, apiErr(d.Log, err)
		}
		if err := d.Groups.RevokeInvite(ctx, p.UserID, groupID, inviteID); err != nil {
			return nil, apiErr(d.Log, err)
		}
		return &emptyOutput{}, nil
	})
}
