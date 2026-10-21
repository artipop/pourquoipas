package integrationtests

import (
	"net/http"
	"testing"

	"github.com/artipop/pourquoipas/server/client"
	"github.com/artipop/pourquoipas/server/model"

	"github.com/stretchr/testify/require"
)

// Who administers a hosted install, and how somebody becomes that. The shape is
// every self-hosted board's: the first person through the door owns it, and
// after that the door needs an invite.

func TestTheFirstPersonToRegisterAdministersTheInstall(t *testing.T) {
	th := SetupTestHelperLocalMode(t)
	defer th.TearDown()
	th.Client = client.NewClient(th.Server.Config().ServerRoot, "")

	th.RegisterAndLogin(th.Client, "first", "first@sample.com", password, "")

	me, resp := th.Client.GetMe()
	th.CheckOK(resp)
	require.Equal(t, model.RoleAdmin, me.Role)
	require.Contains(t, me.Permissions, model.PermissionManageSystem.Id)
}

func TestTheSecondPersonIsAnOrdinaryMember(t *testing.T) {
	th := SetupTestHelperLocalMode(t)
	defer th.TearDown()
	th.Client = client.NewClient(th.Server.Config().ServerRoot, "")

	th.RegisterAndLogin(th.Client, "first", "first@sample.com", password, "")
	team, resp := th.Client.GetTeam(model.GlobalTeamID)
	th.CheckOK(resp)

	second := client.NewClient(th.Server.Config().ServerRoot, "")
	th.RegisterAndLogin(second, "second", "second@sample.com", password, team.SignupToken)

	me, resp := second.GetMe()
	th.CheckOK(resp)
	require.Equal(t, model.RoleMember, me.Role)
	require.NotContains(t, me.Permissions, model.PermissionManageSystem.Id)
}

// Without a token the door is open only while nobody has claimed the install —
// and "claimed" is counted in admins, not in accounts, because this app makes
// one per agent and per source.
func TestRegisteringWithoutATokenIsRefusedOnceThereIsAnAdmin(t *testing.T) {
	th := SetupTestHelperLocalMode(t)
	defer th.TearDown()
	th.Client = client.NewClient(th.Server.Config().ServerRoot, "")

	th.RegisterAndLogin(th.Client, "first", "first@sample.com", password, "")

	_, resp := th.Client.Register(&model.RegisterRequest{
		Username: "gatecrasher",
		Email:    "gatecrasher@sample.com",
		Password: password,
	})
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// An account this app makes for an agent must never be able to claim an install
// somebody is about to register on.
func TestAnAgentAccountDoesNotBecomeTheAdmin(t *testing.T) {
	th := SetupTestHelperLocalMode(t)
	defer th.TearDown()
	th.Client = client.NewClient(th.Server.Config().ServerRoot, "")

	require.NoError(t, th.Server.App().RegisterMachineUser("klaus", "klaus@agents.invalid", password, model.UserKindAgent))

	th.RegisterAndLogin(th.Client, "first", "first@sample.com", password, "")

	me, resp := th.Client.GetMe()
	th.CheckOK(resp)
	require.Equal(t, model.RoleAdmin, me.Role, "the person who registered should own the install, not the agent that predates them")
}

func TestOnlyAnAdminManagesAccounts(t *testing.T) {
	th := SetupTestHelperLocalMode(t)
	defer th.TearDown()
	clients := setupLocalClients(th)

	users, resp := th.Client.GetAdminUsers()
	th.CheckOK(resp)
	require.NotEmpty(t, users)

	_, resp = clients.Admin.GetAdminUsers()
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "an admin of a board is not an admin of the install")

	_, resp = clients.Admin.SetUserRole(clients.TeamMember.GetUserID(), model.RoleAdmin)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	_, resp = clients.Admin.SetUserActive(clients.TeamMember.GetUserID(), false)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// An install nobody can administer cannot be repaired from inside the product:
// there is no console and no unix socket any more.
func TestTheLastAdminCannotBeGivenAway(t *testing.T) {
	th := SetupTestHelperLocalMode(t)
	defer th.TearDown()
	clients := setupLocalClients(th)

	_, resp := th.Client.SetUserRole(th.Client.GetUserID(), model.RoleMember)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	_, resp = th.Client.SetUserActive(th.Client.GetUserID(), false)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// With somebody else holding the job, stepping down is allowed.
	_, resp = th.Client.SetUserRole(clients.TeamMember.GetUserID(), model.RoleAdmin)
	th.CheckOK(resp)

	_, resp = th.Client.SetUserRole(th.Client.GetUserID(), model.RoleMember)
	th.CheckOK(resp)

	_, resp = th.Client.GetAdminUsers()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	_, resp = clients.TeamMember.GetAdminUsers()
	th.CheckOK(resp)
}

// Turning an account off has to end the sessions already open in somebody's
// browser: every user read filters delete_at, so login stops working by itself,
// but a live session would go on working for three days.
func TestDeactivatingAnAccountEndsItsSessions(t *testing.T) {
	th := SetupTestHelperLocalMode(t)
	defer th.TearDown()
	clients := setupLocalClients(th)

	// Read the id while the account still answers: the test client's GetUserID
	// is a live GetMe, and that is exactly what deactivating takes away.
	member, resp := clients.TeamMember.GetMe()
	th.CheckOK(resp)

	_, resp = th.Client.SetUserActive(member.ID, false)
	th.CheckOK(resp)

	_, resp = clients.TeamMember.GetMe()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_, resp = th.Client.SetUserActive(member.ID, true)
	th.CheckOK(resp)
}
