package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
	"uuid"

	"github.com/danielgtaylor/huma/v2"

	"github.com/zarnautovic/tdeemon/internal/domain/user"
	"github.com/zarnautovic/tdeemon/internal/notify"
	"github.com/zarnautovic/tdeemon/internal/push"
	"github.com/zarnautovic/tdeemon/internal/store"
)

// A household (story 408): users of the instance who keep one kitchen,
// every food and recipe of a member each member's. A web session invites
// another user into it by their address, revokes the invite, or, as the
// one invited, declines it, sees what joining would do and joins; a
// member leaves. Its members see one another's name and address, and
// nothing else of one another's but the foods and recipes. A
// notification on their devices tells the one invited, the members of
// one who joined, and those who stay of one who left.

// householdPage is the web app's page of the household, which a
// household's notification opens.
const householdPage = "/settings/household"

// The codes of a household's refusals, the value of the problem's error
// at its location: a client tells them apart by it, as their statuses
// alone do not.
const (
	// householdNoUser: no user of the instance has the address.
	householdNoUser = "no_user"
	// householdSelf: the address is the user's own.
	householdSelf = "self"
	// householdDeactivated: the admin deactivated the user of the address.
	householdDeactivated = "deactivated"
	// householdMember: they are in the household already.
	householdMember = "member"
	// householdOther: they (the invited user, of an accept) are in
	// another household, which they leave first.
	householdOther = "other_household"
	// householdWaiting: an invite of them from the household waits already.
	householdWaiting = "waiting"
	// householdCycle: the merge would make a recipe its own ingredient.
	householdCycle = "recipe_cycle"
	// householdChanged: the merge is no longer the one its preview showed.
	householdChanged = "merge_changed"
)

// householdPersonBody is a user an invite names, or a member: their name
// and address.
type householdPersonBody struct {
	Name  string `json:"name"`
	Email string `json:"email" format:"email"`
}

// householdMemberBody is a member as the others see them: their name
// and address alone.
type householdMemberBody struct {
	Name  string `json:"name"`
	Email string `json:"email" format:"email"`
	You   bool   `json:"you,omitempty" doc:"The member is you."`
}

type householdInviteBody struct {
	ID        string              `json:"id" format:"uuid" doc:"What a revoke, a decline, a preview and an accept name."`
	By        householdPersonBody `json:"by" doc:"Who invited: a member of the household, or a user in none, whom the accept makes a household with."`
	Invited   householdPersonBody `json:"invited"`
	CreatedAt time.Time           `json:"created_at"`
	ExpiresAt time.Time           `json:"expires_at" doc:"When it stops waiting, unanswered: 7 days after it was made."`
}

func newHouseholdInviteBody(inv *store.HouseholdInvite) householdInviteBody {
	return householdInviteBody{
		ID:        inv.ID.String(),
		By:        householdPersonBody{Name: inv.By.Name, Email: inv.By.Email},
		Invited:   householdPersonBody{Name: inv.Invited.Name, Email: inv.Invited.Email},
		CreatedAt: inv.CreatedAt.UTC(),
		ExpiresAt: inv.ExpiresAt.UTC(),
	}
}

type householdBody struct {
	Members  []householdMemberBody `json:"members" nullable:"false" doc:"The household's members, you among them, in the order they joined; none when you are in no household."`
	Sent     []householdInviteBody `json:"sent" nullable:"false" doc:"The invites waiting that the household made, any of its members (yours alone when you are in none), the newest first."`
	Received []householdInviteBody `json:"received" nullable:"false" doc:"The invites into a household waiting for your answer, the newest first."`
}

type householdOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         householdBody
}

type householdInviteOutput struct {
	Body householdInviteBody
}

type inviteToHouseholdInput struct {
	Body struct {
		Email string `json:"email" format:"email" maxLength:"254" doc:"The address of the user of the instance to invite."`
	}
}

type householdInviteInput struct {
	ID string `path:"id" format:"uuid" doc:"The invite's id."`
}

type acceptHouseholdInput struct {
	ID   string `path:"id" format:"uuid" doc:"The invite's id."`
	Body struct {
		Fingerprint string `json:"fingerprint" minLength:"1" maxLength:"64" doc:"The preview's fingerprint: the accept does the merge it showed, or none."`
	}
}

type mergeCountsBody struct {
	Foods   int `json:"foods" doc:"Foods that are no recipes."`
	Recipes int `json:"recipes"`
}

type mergeRenameBody struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type mergeBarcodeBody struct {
	Food string `json:"food" doc:"The food of yours, by its name after the merge."`
	Code string `json:"code"`
}

// mergeReportBody is what joining a household does to the foods.
type mergeReportBody struct {
	Merged   mergeCountsBody    `json:"merged" doc:"Your foods and recipes that become one with the household's: the household's values, your note, aliases and archived."`
	Received mergeCountsBody    `json:"received" doc:"The household's that come to you."`
	Given    mergeCountsBody    `json:"given" doc:"Yours that go to the household's members."`
	Renamed  []mergeRenameBody  `json:"renamed" nullable:"false" doc:"Foods of yours renamed, as the household has a food of their name."`
	Dropped  []mergeBarcodeBody `json:"dropped" nullable:"false" doc:"Barcodes dropped from a food of yours, as another food has them."`
}

func newMergeReportBody(r *store.MergeReport) mergeReportBody {
	counts := func(c store.MergeCounts) mergeCountsBody { return mergeCountsBody{Foods: c.Foods, Recipes: c.Recipes} }
	b := mergeReportBody{
		Merged: counts(r.Merged), Received: counts(r.Received), Given: counts(r.Given),
		Renamed: make([]mergeRenameBody, 0, len(r.Renamed)), Dropped: make([]mergeBarcodeBody, 0, len(r.Dropped)),
	}
	for _, n := range r.Renamed {
		b.Renamed = append(b.Renamed, mergeRenameBody{From: n.From, To: n.To})
	}
	for _, d := range r.Dropped {
		b.Dropped = append(b.Dropped, mergeBarcodeBody{Food: d.Food, Code: d.Code})
	}
	return b
}

type mergePreviewOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		Report      mergeReportBody `json:"report"`
		Fingerprint string          `json:"fingerprint" doc:"What the accept is given, to do this merge and no other."`
	}
}

func (s *sessions) registerHousehold(a huma.API) {
	session := []int{http.StatusUnauthorized, http.StatusForbidden}
	huma.Register(a, huma.Operation{
		OperationID: "get-household",
		Method:      http.MethodGet,
		Path:        Prefix + "/household",
		Summary:     "Get your household",
		Description: "Needs a web session. Its members, by name and address, and the invites waiting: those the household made, and those for your answer.",
		Tags:        []string{"household"},
		Errors:      session,
	}, s.getHousehold)
	huma.Register(a, huma.Operation{
		OperationID: "invite-to-household",
		Method:      http.MethodPost,
		Path:        Prefix + "/household/invites",
		Summary:     "Invite a user into your household",
		Description: "Needs a web session. Invites the user of the address into your household, or into one of the two of you when you are in none, " +
			"for 7 days; they are told by a notification on their devices. A refusal's error at body.email has a code as its value: " +
			"no_user, self or deactivated (422: no user of the instance has the address, it is yours, or the admin deactivated its user); " +
			"member, other_household or waiting (409: they are in your household already, they are in another, which they leave first, " +
			"or an invite of them from your household waits).",
		Tags:          []string{"household"},
		DefaultStatus: http.StatusCreated,
		Errors:        append(session, http.StatusConflict, http.StatusUnprocessableEntity),
	}, s.inviteToHousehold)
	huma.Register(a, huma.Operation{
		OperationID:   "delete-household-invite",
		Method:        http.MethodDelete,
		Path:          Prefix + "/household/invites/{id}",
		Summary:       "Revoke or decline an invite",
		Description:   "Needs a web session. An invite your household made is revoked, by any of its members; one for your answer is declined.",
		Tags:          []string{"household"},
		DefaultStatus: http.StatusNoContent,
		Errors:        append(session, http.StatusNotFound),
	}, s.deleteHouseholdInvite)
	acceptErrors := "A refusal's error at path.id has a code as its value: member or other_household (you are in that household already, " +
		"or in another, which you leave first), or recipe_cycle (the merge would make a recipe its own ingredient)."
	huma.Register(a, huma.Operation{
		OperationID: "preview-household-invite",
		Method:      http.MethodGet,
		Path:        Prefix + "/household/invites/{id}/preview",
		Summary:     "See what joining a household does",
		Description: "Needs a web session; of an invite for your answer. What accepting it would do now, the merge itself done and rolled back: " +
			"how many of your foods and recipes become one with the household's, how many come to you and how many go to it, and each food " +
			"of yours renamed or losing a barcode. " + acceptErrors,
		Tags:   []string{"household"},
		Errors: append(session, http.StatusNotFound, http.StatusConflict),
	}, s.previewHouseholdInvite)
	huma.Register(a, huma.Operation{
		OperationID: "accept-household-invite",
		Method:      http.MethodPost,
		Path:        Prefix + "/household/invites/{id}/accept",
		Summary:     "Join a household",
		Description: "Needs a web session; of an invite for your answer. Does the merge its preview showed, of the fingerprint given, and makes you " +
			"a member: from then on every food and recipe of a member is every member's, and your meals, measurements and plan stay yours. " +
			"The members are told by a notification on their devices. Joining is never undone. A merge that is no longer the one the preview " +
			"showed (the household's foods or yours changed since) does nothing: a 409 whose error at body.fingerprint has the code " +
			"merge_changed, and a new preview shows the merge now. " + acceptErrors + " Answers your household.",
		Tags:   []string{"household"},
		Errors: append(session, http.StatusNotFound, http.StatusConflict),
	}, s.acceptHouseholdInvite)
	huma.Register(a, huma.Operation{
		OperationID: "leave-household",
		Method:      http.MethodPost,
		Path:        Prefix + "/household/leave",
		Summary:     "Leave your household",
		Description: "Needs a web session. Every member keeps every food and recipe they have, yours as ordinary foods of your own; " +
			"a household of one member left is no more. The members who stay are told by a notification on their devices. " +
			"Leaving is never undone: an invite brings you back. You in no household is a 404.",
		Tags:          []string{"household"},
		DefaultStatus: http.StatusNoContent,
		Errors:        append(session, http.StatusNotFound),
	}, s.leaveHousehold)
}

// householdRefusal is a refusal of status, detail, whose error at where
// has the code as its value.
func householdRefusal(status int, detail, where, code, why string) error {
	return huma.NewError(status, detail, &huma.ErrorDetail{Location: where, Message: why, Value: ownValue(code)})
}

func (s *sessions) getHousehold(ctx context.Context, _ *struct{}) (*householdOutput, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return nil, err
	}
	b, err := s.household(ctx, p.User.ID)
	if err != nil {
		return nil, internalError("reading the household failed", err)
	}
	return &householdOutput{CacheControl: "no-store", Body: b}, nil
}

// household is userID's household as get-household answers it.
func (s *sessions) household(ctx context.Context, userID uuid.UUID) (householdBody, error) {
	b := householdBody{Members: []householdMemberBody{}, Sent: []householdInviteBody{}, Received: []householdInviteBody{}}
	members, err := s.db.HouseholdMembers(ctx, userID)
	if err != nil {
		return householdBody{}, err
	}
	for i := range members {
		m := &members[i]
		b.Members = append(b.Members, householdMemberBody{Name: m.Name, Email: m.Email, You: m.ID == userID})
	}
	sent, err := s.db.HouseholdInvitesFrom(ctx, userID, s.now())
	if err != nil {
		return householdBody{}, err
	}
	for i := range sent {
		b.Sent = append(b.Sent, newHouseholdInviteBody(&sent[i]))
	}
	received, err := s.db.HouseholdInvitesTo(ctx, userID, s.now())
	if err != nil {
		return householdBody{}, err
	}
	for i := range received {
		b.Received = append(b.Received, newHouseholdInviteBody(&received[i]))
	}
	return b, nil
}

func (s *sessions) inviteToHousehold(ctx context.Context, in *inviteToHouseholdInput) (*householdInviteOutput, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return nil, err
	}
	const refused = "the invite is refused"
	refuse := func(status int, code, why string) error {
		return householdRefusal(status, refused, "body.email", code, why)
	}
	// The instance's users know one another, as a recipe's share does: an
	// address of none says so.
	invited, err := s.db.UserByEmail(ctx, user.NormalizeEmail(in.Body.Email))
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, refuse(http.StatusUnprocessableEntity, householdNoUser, "no user of this instance has that address")
	case err != nil:
		return nil, internalError("inviting failed", err)
	case invited.ID == p.User.ID:
		return nil, refuse(http.StatusUnprocessableEntity, householdSelf, "that is your own address: invite someone else")
	case invited.DisabledAt != nil:
		return nil, refuse(http.StatusUnprocessableEntity, householdDeactivated, "the admin deactivated that user")
	}
	inv, err := s.db.InviteToHousehold(ctx, p.User.ID, invited.ID, s.now())
	switch {
	case errors.Is(err, store.ErrNotFound):
		// Deleted since the lookup.
		return nil, refuse(http.StatusUnprocessableEntity, householdNoUser, "no user of this instance has that address")
	case errors.Is(err, store.ErrInviteSelf):
		return nil, refuse(http.StatusUnprocessableEntity, householdSelf, "that is your own address: invite someone else")
	case errors.Is(err, store.ErrInviteMember):
		return nil, refuse(http.StatusConflict, householdMember, "they are in your household already")
	case errors.Is(err, store.ErrInviteOtherHousehold):
		return nil, refuse(http.StatusConflict, householdOther, "they are in another household: they leave it first")
	case errors.Is(err, store.ErrInviteWaiting):
		return nil, refuse(http.StatusConflict, householdWaiting, "an invite of theirs from your household waits already")
	case err != nil:
		return nil, internalError("inviting failed", err)
	}
	s.tellHousehold(ctx, []user.User{invited}, invitedWords, p.User.Name, "household-invite-"+inv.ID.String())
	return &householdInviteOutput{Body: newHouseholdInviteBody(&inv)}, nil
}

func (s *sessions) deleteHouseholdInvite(ctx context.Context, in *householdInviteInput) (*struct{}, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return nil, err
	}
	none := huma.Error404NotFound("no such invite: it expired, or was answered or revoked")
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, none
	}
	// The household's to revoke, else the user's own to decline.
	err = s.db.RevokeHouseholdInvite(ctx, p.User.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		err = s.db.DeclineHouseholdInvite(ctx, p.User.ID, id)
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, none
	case err != nil:
		return nil, internalError("deleting the invite failed", err)
	}
	return nil, nil
}

// joinRefusal is the answer to a preview's or an accept's refusal err,
// or nil for none of them.
func joinRefusal(err error) error {
	const refused = "joining is refused"
	switch {
	case errors.Is(err, store.ErrNotFound):
		return huma.Error404NotFound("no such invite for you: it expired, or was revoked or declined")
	case errors.Is(err, store.ErrInviteMember):
		return householdRefusal(http.StatusConflict, refused, "path.id", householdMember, "you are in that household already")
	case errors.Is(err, store.ErrInviteOtherHousehold):
		return householdRefusal(http.StatusConflict, refused, "path.id", householdOther, "you are in another household: leave it first")
	case errors.Is(err, store.ErrRecipeCycle):
		// Which recipe, the household's perhaps, is not said before the
		// user joins.
		return householdRefusal(http.StatusConflict, refused, "path.id", householdCycle,
			"the merge would make a recipe its own ingredient: change the recipes of one name first")
	case errors.Is(err, store.ErrMergeChanged):
		return householdRefusal(http.StatusConflict, refused, "body.fingerprint", householdChanged,
			"the foods changed since the preview: see the new preview, and accept that one")
	}
	return nil
}

func (s *sessions) previewHouseholdInvite(ctx context.Context, in *householdInviteInput) (*mergePreviewOutput, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, joinRefusal(store.ErrNotFound)
	}
	preview, err := s.db.PreviewHouseholdInvite(ctx, p.User.ID, id, s.now())
	if r := joinRefusal(err); r != nil {
		return nil, r
	} else if err != nil {
		return nil, internalError("previewing the merge failed", err)
	}
	out := &mergePreviewOutput{CacheControl: "no-store"}
	out.Body.Report = newMergeReportBody(&preview.Report)
	out.Body.Fingerprint = preview.Fingerprint
	return out, nil
}

func (s *sessions) acceptHouseholdInvite(ctx context.Context, in *acceptHouseholdInput) (*householdOutput, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, joinRefusal(store.ErrNotFound)
	}
	// Those to tell are the members as the join left them.
	_, mates, err := s.db.AcceptHouseholdInvite(ctx, p.User.ID, actorOf(p), id, in.Body.Fingerprint, s.now())
	if r := joinRefusal(err); r != nil {
		return nil, r
	} else if err != nil {
		return nil, internalError("joining the household failed", err)
	}
	s.tellHousehold(ctx, mates, joinedWords, p.User.Name, "household-"+p.User.ID.String())
	b, err := s.household(ctx, p.User.ID)
	if err != nil {
		return nil, internalError("joining the household failed", err)
	}
	return &householdOutput{CacheControl: "no-store", Body: b}, nil
}

func (s *sessions) leaveHousehold(ctx context.Context, _ *struct{}) (*struct{}, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return nil, err
	}
	// Those to tell are the members who stay, as the leave found them.
	stay, err := s.db.LeaveHousehold(ctx, p.User.ID, actorOf(p), s.now())
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, huma.Error404NotFound("you are in no household")
	case err != nil:
		return nil, internalError("leaving the household failed", err)
	}
	s.tellHousehold(ctx, stay, leftWords, p.User.Name, "household-"+p.User.ID.String())
	return nil, nil
}

// householdWords are a household's notification in a language: its
// title, of a user's name, and its body.
type householdWords struct{ title, body string }

// invitedWords tell the one invited by whom; joinedWords the members who
// joined; leftWords those who stay who left: in each language.
var (
	invitedWords = map[string]householdWords{
		"en": {"%s invited you into their household", "See what joining does to your foods before you answer."},
		"hr": {"%s te poziva u svoje kućanstvo", "Prije odgovora pogledaj što pridruživanje radi s tvojim namirnicama."},
	}
	joinedWords = map[string]householdWords{
		"en": {"%s joined your household", "Your foods and recipes are theirs too now."},
		"hr": {"%s je sada u tvom kućanstvu", "Vaše namirnice i recepti sada su zajednički."},
	}
	leftWords = map[string]householdWords{
		"en": {"%s left your household", "Each of you keeps the foods and recipes you have."},
		"hr": {"%s više nije u tvom kućanstvu", "Svatko zadržava namirnice i recepte koje ima."},
	}
)

// tellHousehold tells each of to, by a notification on their devices and
// in their language, words of who (a user's name), which opens the
// household's page; tag replaces one of the same still on a device. One
// that fails is the logs', never the operation's.
func (s *sessions) tellHousehold(ctx context.Context, to []user.User, words map[string]householdWords, who, tag string) {
	if s.push == nil {
		return
	}
	for i := range to {
		u := &to[i]
		w, ok := words[u.Locale]
		if !ok {
			w = words["en"]
		}
		n := push.Notification{Title: fmt.Sprintf(w.title, who), Body: w.body, URL: householdPage, Tag: tag}
		if r := notify.Push(ctx, s.db, s.push, *u, n, push.Message{TTL: 24 * 60 * 60}, s.now()); r.Err != nil {
			s.logger.WarnContext(ctx, "telling the household failed", "user_id", u.ID.String(), "error", r.Err)
		}
	}
}
