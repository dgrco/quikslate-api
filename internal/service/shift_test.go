package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

// Tests for ShiftService. Two themes run through them: that a rejected
// request performs no write, and that the overlap check is asked the right
// question, since both failures are silent. An authorization bug that still
// writes leaves the damage done behind a 403, and an overlap check given the
// wrong window double-books someone without erroring at all.
//
// The fake records what it was asked rather than only what to answer, so
// tests can assert on the call itself. Checking that AssignShift returned nil
// says nothing about whether it excluded the right shift from the overlap
// query.

// overlapQuery captures one GetOverlappingShiftsForUser call. The window and
// the excluded id are the parts callers get wrong.
type overlapQuery struct {
	userId         string
	from, to       time.Time
	excludeShiftId *string
}

// fakeShiftRepo is a domain.Repo test double. domain.Repo embeds every
// sub-repo, so the nil embedded interface supplies the rest of the method
// set; only the methods ShiftService actually reaches are overridden. Any
// other call nil-panics, which is the intent: it surfaces an unexpected
// dependency instead of quietly returning a zero value.
type fakeShiftRepo struct {
	domain.Repo

	shift             domain.Shift
	shiftErr          error
	location          domain.Location
	locationErr       error
	position          domain.Position
	positionErr       error
	businessMember    domain.BusinessMember
	businessMemberErr error
	locationRole      domain.LocationRole
	locationRoleErr   error
	overlapping       []domain.Shift

	// Recorded calls.
	lastOverlapQuery *overlapQuery
	lastManagerView  *bool
	created          bool
	updated          bool
	assigned         bool
	unassigned       bool
	cancelled        bool
	deleted          bool
}

// wrote reports whether any mutating repo method ran. Authorization tests
// assert this stays false: returning an error while having already written
// is the failure mode worth catching.
func (f *fakeShiftRepo) wrote() bool {
	return f.created || f.updated || f.assigned || f.unassigned || f.cancelled || f.deleted
}

func (f *fakeShiftRepo) GetShiftById(ctx context.Context, id string) (domain.Shift, error) {
	return f.shift, f.shiftErr
}

func (f *fakeShiftRepo) GetLocationById(ctx context.Context, id string) (domain.Location, error) {
	return f.location, f.locationErr
}

func (f *fakeShiftRepo) GetPositionById(ctx context.Context, id string) (domain.Position, error) {
	return f.position, f.positionErr
}

func (f *fakeShiftRepo) GetBusinessMember(ctx context.Context, userId, businessId string) (domain.BusinessMember, error) {
	return f.businessMember, f.businessMemberErr
}

func (f *fakeShiftRepo) GetLocationRole(ctx context.Context, userId, locationId, businessId string) (domain.LocationRole, error) {
	return f.locationRole, f.locationRoleErr
}

func (f *fakeShiftRepo) GetOverlappingShiftsForUser(
	ctx context.Context,
	userId string,
	from, to time.Time,
	excludeShiftId *string,
) ([]domain.Shift, error) {
	f.lastOverlapQuery = &overlapQuery{userId: userId, from: from, to: to, excludeShiftId: excludeShiftId}
	return f.overlapping, nil
}

func (f *fakeShiftRepo) GetShiftDetailsByLocationId(
	ctx context.Context,
	locationId string,
	from, to time.Time,
	managerView bool,
) ([]domain.ShiftDetail, error) {
	f.lastManagerView = &managerView
	return nil, nil
}

func (f *fakeShiftRepo) CreateShift(
	ctx context.Context,
	userId *string,
	locationId, positionId string,
	status domain.ShiftStatus,
	startTime, endTime time.Time,
) (domain.Shift, error) {
	f.created = true
	return domain.Shift{Id: "new-shift"}, nil
}

func (f *fakeShiftRepo) UpdateShiftById(ctx context.Context, id string, update domain.ShiftUpdate) error {
	f.updated = true
	return nil
}

func (f *fakeShiftRepo) AssignShift(ctx context.Context, id, userId string) error {
	f.assigned = true
	return nil
}

func (f *fakeShiftRepo) UnassignShift(ctx context.Context, id string) error {
	f.unassigned = true
	return nil
}

func (f *fakeShiftRepo) CancelShift(ctx context.Context, id string) error {
	f.cancelled = true
	return nil
}

func (f *fakeShiftRepo) DeleteShift(ctx context.Context, id string) error {
	f.deleted = true
	return nil
}

const (
	testBusinessId = "business-1"
	testLocationId = "location-1"
	testShiftId    = "shift-1"
	testCallerId   = "caller-1"
	testTargetId   = "target-1"
)

var (
	shiftStart = time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	shiftEnd   = time.Date(2026, 4, 6, 17, 0, 0, 0, time.UTC)
)

// shiftCtx builds the context RequireLocationMember would have populated for
// a location-scoped request.
func shiftCtx(role domain.LRole, isAdmin bool) context.Context {
	ctx := context.WithValue(context.Background(), ctxkeys.UserId, testCallerId)
	ctx = context.WithValue(ctx, ctxkeys.BusinessId, testBusinessId)
	ctx = context.WithValue(ctx, ctxkeys.LocationId, testLocationId)
	ctx = context.WithValue(ctx, ctxkeys.IsAdmin, isAdmin)
	ctx = context.WithValue(ctx, ctxkeys.Role, role)
	return ctx
}

// newFakeRepo returns a repo where every lookup succeeds and nothing
// overlaps, so each test only has to set up the one thing it is about.
func newFakeRepo() *fakeShiftRepo {
	return &fakeShiftRepo{
		shift: domain.Shift{
			Id:         testShiftId,
			LocationId: testLocationId,
			PositionId: "position-1",
			Status:     domain.Draft,
			StartTime:  shiftStart,
			EndTime:    shiftEnd,
		},
		location:       domain.Location{Id: testLocationId, BusinessId: testBusinessId},
		position:       domain.Position{Id: "position-1", BusinessId: testBusinessId},
		businessMember: domain.BusinessMember{UserId: testTargetId, BusinessId: testBusinessId},
		locationRole: domain.LocationRole{
			UserId: testTargetId, BusinessId: testBusinessId,
			LocationId: testLocationId, Role: domain.Employee,
		},
	}
}

// Employees get the published schedule; drafts and unfilled slots belong to
// whoever is building it. That filtering happens in SQL, driven entirely by
// the managerView argument, so passing the wrong value here leaks unpublished
// plans to staff. Nothing downstream would catch it: the query succeeds and
// returns rows either way.
func TestGetShiftsByLocationManagerView(t *testing.T) {
	cases := []struct {
		name            string
		role            domain.LRole
		isAdmin         bool
		wantManagerView bool
	}{
		{"business admin", domain.EmptyRole, true, true},
		{"location lead", domain.LocationLead, false, true},
		{"manager", domain.Manager, false, true},
		{"employee sees only the published schedule", domain.Employee, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			svc := NewShiftService(repo)

			_, err := svc.GetShiftsByLocation(shiftCtx(tc.role, tc.isAdmin), shiftStart, shiftStart.Add(7*24*time.Hour))
			if err != nil {
				t.Fatalf("GetShiftsByLocation() error = %v", err)
			}
			if repo.lastManagerView == nil {
				t.Fatal("repo was never queried")
			}
			if *repo.lastManagerView != tc.wantManagerView {
				t.Errorf("managerView = %v, want %v", *repo.lastManagerView, tc.wantManagerView)
			}
		})
	}
}

// An out-of-bounds range must be rejected before it reaches the database,
// which is the only thing keeping the endpoint from being turned back into an
// unbounded dump of every shift ever scheduled.
func TestGetShiftsByLocationRejectsOversizedRange(t *testing.T) {
	repo := newFakeRepo()
	svc := NewShiftService(repo)

	_, err := svc.GetShiftsByLocation(
		shiftCtx(domain.Manager, false),
		shiftStart,
		shiftStart.Add(domain.MaxShiftRange+time.Hour),
	)
	if err == nil {
		t.Fatal("GetShiftsByLocation() error = nil, want a range rejection")
	}
	if repo.lastManagerView != nil {
		t.Error("repo was queried despite the range being rejected")
	}
}

func TestCreateShiftRejectsOverlap(t *testing.T) {
	repo := newFakeRepo()
	repo.overlapping = []domain.Shift{{Id: "existing-shift"}}
	svc := NewShiftService(repo)

	target := testTargetId
	_, err := svc.CreateShift(shiftCtx(domain.Manager, false), "position-1", &target, domain.Assigned, shiftStart, shiftEnd)

	if !errors.Is(err, domain.ErrShiftOverlap) {
		t.Errorf("CreateShift() error = %v, want ErrShiftOverlap", err)
	}
	if repo.created {
		t.Error("shift was created despite overlapping an existing one")
	}
}

// An unassigned shift belongs to nobody, so there is no one to double-book.
// Querying anyway would be wrong rather than merely wasteful: the query keys
// on a user id, and there isn't one.
func TestCreateShiftWithoutAssigneeSkipsOverlapCheck(t *testing.T) {
	repo := newFakeRepo()
	svc := NewShiftService(repo)

	_, err := svc.CreateShift(shiftCtx(domain.Manager, false), "position-1", nil, domain.Uncovered, shiftStart, shiftEnd)
	if err != nil {
		t.Fatalf("CreateShift() error = %v", err)
	}
	if repo.lastOverlapQuery != nil {
		t.Error("overlap was checked for a shift with no assignee")
	}
	if !repo.created {
		t.Error("shift was not created")
	}
}

// The overlap query must exclude the shift being assigned, or promoting a
// draft that already carries a user_id fails against itself. That path is
// the only way to promote such a draft, since PATCH refuses to set status
// "assigned". A regression here would look like an unexplained conflict on a
// perfectly valid assignment.
func TestAssignShiftExcludesItselfFromOverlapCheck(t *testing.T) {
	repo := newFakeRepo()
	svc := NewShiftService(repo)

	if err := svc.AssignShift(shiftCtx(domain.Manager, false), testShiftId, testTargetId); err != nil {
		t.Fatalf("AssignShift() error = %v", err)
	}

	q := repo.lastOverlapQuery
	if q == nil {
		t.Fatal("overlap was never checked")
	}
	if q.excludeShiftId == nil {
		t.Fatal("overlap check did not exclude any shift, so assigning a pre-assigned draft would conflict with itself")
	}
	if *q.excludeShiftId != testShiftId {
		t.Errorf("excluded shift = %q, want %q", *q.excludeShiftId, testShiftId)
	}
	if q.userId != testTargetId {
		t.Errorf("overlap checked against user %q, want the assignee %q", q.userId, testTargetId)
	}
}

// Moving one end of an assigned shift can push it onto another of that
// person's shifts, so the re-check has to use the window the shift is moving
// *to*: the changed end plus the existing value for the end that is staying
// put. Checking the old window would let a move land straight on top of
// another shift.
func TestUpdateShiftRechecksOverlapAgainstNewWindow(t *testing.T) {
	repo := newFakeRepo()
	assignee := testTargetId
	repo.shift.UserId = &assignee
	repo.shift.Status = domain.Assigned
	svc := NewShiftService(repo)

	newEnd := shiftEnd.Add(3 * time.Hour)
	err := svc.UpdateShift(shiftCtx(domain.LocationLead, false), testShiftId, domain.ShiftUpdate{EndTime: &newEnd})
	if err != nil {
		t.Fatalf("UpdateShift() error = %v", err)
	}

	q := repo.lastOverlapQuery
	if q == nil {
		t.Fatal("overlap was never re-checked after a time change")
	}
	if !q.from.Equal(shiftStart) {
		t.Errorf("overlap window start = %v, want the unchanged start %v", q.from, shiftStart)
	}
	if !q.to.Equal(newEnd) {
		t.Errorf("overlap window end = %v, want the new end %v", q.to, newEnd)
	}
}

// A partial update sets one end and leaves the other, so validity is a
// question about the combination of the new value and the stored one. A check
// that only looked at the supplied field would accept an end time before the
// existing start.
func TestUpdateShiftValidatesPartialTimesAgainstStoredValues(t *testing.T) {
	cases := []struct {
		name    string
		start   *time.Time
		end     *time.Time
		wantErr bool
	}{
		{"new end after stored start", nil, timePtr(shiftEnd.Add(time.Hour)), false},
		{"new end before stored start", nil, timePtr(shiftStart.Add(-time.Hour)), true},
		{"new end exactly at stored start leaves no duration", nil, timePtr(shiftStart), true},

		{"new start before stored end", timePtr(shiftStart.Add(-time.Hour)), nil, false},
		{"new start after stored end", timePtr(shiftEnd.Add(time.Hour)), nil, true},
		{"new start exactly at stored end leaves no duration", timePtr(shiftEnd), nil, true},

		{"both supplied and ordered", timePtr(shiftStart), timePtr(shiftEnd), false},
		{"both supplied and reversed", timePtr(shiftEnd), timePtr(shiftStart), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			svc := NewShiftService(repo)

			err := svc.UpdateShift(
				shiftCtx(domain.Manager, false),
				testShiftId,
				domain.ShiftUpdate{StartTime: tc.start, EndTime: tc.end},
			)

			if tc.wantErr {
				if !errors.Is(err, domain.ErrInvalidShiftTimes) {
					t.Errorf("UpdateShift() error = %v, want ErrInvalidShiftTimes", err)
				}
				if repo.updated {
					t.Error("shift was updated despite invalid times")
				}
			} else if err != nil {
				t.Errorf("UpdateShift() error = %v, want nil", err)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time { return new(t) }

// Unassigning a shift nobody holds is a no-op dressed as a request. It
// reports ErrNotFound so the caller learns the state was not what they
// thought, rather than getting a success for work that never happened.
func TestUnassignShiftWithNoAssignee(t *testing.T) {
	repo := newFakeRepo()
	repo.shift.UserId = nil
	svc := NewShiftService(repo)

	err := svc.UnassignShift(shiftCtx(domain.Manager, false), testShiftId)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("UnassignShift() error = %v, want ErrNotFound", err)
	}
	if repo.unassigned {
		t.Error("unassign ran against a shift with no assignee")
	}
}

// Every write path, checked against every role. The assertion that matters
// is the second one: a denied caller must leave no trace, since an
// authorization check that runs after the write has already failed.
func TestShiftWriteAuthorization(t *testing.T) {
	callers := []struct {
		name    string
		role    domain.LRole
		isAdmin bool
	}{
		{"admin", domain.EmptyRole, true},
		{"location lead", domain.LocationLead, false},
		{"manager", domain.Manager, false},
		{"employee", domain.Employee, false},
		{"no role at this location", domain.EmptyRole, false},
	}

	// wantAllowed maps caller name to whether the operation should succeed.
	ops := []struct {
		name        string
		run         func(svc *ShiftService, ctx context.Context) error
		wantAllowed map[string]bool
	}{
		{
			name: "create",
			run: func(svc *ShiftService, ctx context.Context) error {
				_, err := svc.CreateShift(ctx, "position-1", nil, domain.Uncovered, shiftStart, shiftEnd)
				return err
			},
			wantAllowed: map[string]bool{"admin": true, "location lead": true, "manager": true},
		},
		{
			name: "update",
			run: func(svc *ShiftService, ctx context.Context) error {
				return svc.UpdateShift(ctx, testShiftId, domain.ShiftUpdate{Status: statusPtr(domain.Covered)})
			},
			wantAllowed: map[string]bool{"admin": true, "location lead": true, "manager": true},
		},
		{
			name: "cancel",
			run: func(svc *ShiftService, ctx context.Context) error {
				return svc.CancelShift(ctx, testShiftId)
			},
			wantAllowed: map[string]bool{"admin": true, "location lead": true, "manager": true},
		},
		{
			// The hard delete is admin-only, unlike cancel's soft delete.
			// requireLocationRole with no roles listed is what expresses
			// that, and it reads as an oversight rather than a rule, so it
			// is worth pinning explicitly.
			name: "delete",
			run: func(svc *ShiftService, ctx context.Context) error {
				return svc.DeleteShift(ctx, testShiftId)
			},
			wantAllowed: map[string]bool{"admin": true},
		},
	}

	for _, op := range ops {
		for _, caller := range callers {
			t.Run(op.name+"/"+caller.name, func(t *testing.T) {
				repo := newFakeRepo()
				svc := NewShiftService(repo)

				err := op.run(svc, shiftCtx(caller.role, caller.isAdmin))
				allowed := op.wantAllowed[caller.name]

				if allowed {
					if err != nil {
						t.Errorf("%s: error = %v, want nil", op.name, err)
					}
					return
				}
				if !errors.Is(err, domain.ErrForbidden) {
					t.Errorf("%s: error = %v, want ErrForbidden", op.name, err)
				}
				if repo.wrote() {
					t.Errorf("%s was denied but still wrote to the repo", op.name)
				}
			})
		}
	}
}

func statusPtr(s domain.ShiftStatus) *domain.ShiftStatus { return new(s) }

// A shift is reached by id, so its location is whatever the row says, not
// whatever the caller's session claims. Without re-deriving authorization
// from the stored locationId, a manager could pass any shift id and act on
// another location's schedule, which the middleware cannot catch because it
// only ever sees the caller's own {locationId}.
func TestShiftFromAnotherLocationIsRejected(t *testing.T) {
	repo := newFakeRepo()
	repo.shift.LocationId = "location-2"
	repo.location = domain.Location{Id: "location-2", BusinessId: testBusinessId}
	svc := NewShiftService(repo)

	err := svc.CancelShift(shiftCtx(domain.Manager, false), testShiftId)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("CancelShift() error = %v, want ErrForbidden", err)
	}
	if repo.cancelled {
		t.Error("shift at another location was cancelled")
	}
}

// The same rule one level up: a shift whose location belongs to a different
// business is out of reach even for an admin, who is otherwise unrestricted
// across their own locations. This is the tenant boundary.
func TestShiftFromAnotherBusinessIsRejectedEvenForAdmin(t *testing.T) {
	repo := newFakeRepo()
	repo.location = domain.Location{Id: testLocationId, BusinessId: "business-2"}
	svc := NewShiftService(repo)

	err := svc.CancelShift(shiftCtx(domain.EmptyRole, true), testShiftId)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("CancelShift() error = %v, want ErrForbidden", err)
	}
	if repo.cancelled {
		t.Error("shift belonging to another business was cancelled")
	}
}
