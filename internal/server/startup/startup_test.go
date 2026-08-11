package startup

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/configuration"
	"github.com/FreekingDean/gojellyfin/internal/store"
	configurationmodal "github.com/FreekingDean/gojellyfin/internal/store/configuration"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	server *Server
	config *config.Service
	users  *users.Service
	client *store.Client
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	system := snapshot(t, client, configuration.SystemConfigurationKey)
	network := snapshot(t, client, networkConfigurationKey)

	t.Cleanup(func() {
		restore(t, client, configuration.SystemConfigurationKey, system)
		restore(t, client, networkConfigurationKey, network)
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	configService := config.New(client)
	usersService := users.New(client)

	return &fixture{
		server: New(configService, usersService),
		config: configService,
		users:  usersService,
		client: client,
	}
}

func snapshot(t *testing.T, client *store.Client, key string) json.RawMessage {
	t.Helper()

	value, err := config.New(client).Configuration(context.Background(), key)
	if err != nil {
		t.Fatalf("failed to read the %s configuration: %v", key, err)
	}

	return value
}

func restore(t *testing.T, client *store.Client, key string, value json.RawMessage) {
	t.Helper()

	ctx := context.Background()
	if value == nil {
		if _, err := client.Configuration.Delete().Where(configurationmodal.Key(key)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the %s configuration: %v", key, err)
		}

		return
	}
	if err := config.New(client).SetConfiguration(ctx, key, value); err != nil {
		t.Errorf("failed to restore the %s configuration: %v", key, err)
	}
}

func (f *fixture) markCompleted(t *testing.T, completed bool) {
	t.Helper()

	value, err := json.Marshal(api.ServerConfiguration{IsStartupWizardCompleted: apiutil.Ptr(completed)})
	if err != nil {
		t.Fatalf("failed to encode the configuration: %v", err)
	}
	if err := f.config.SetConfiguration(context.Background(), configuration.SystemConfigurationKey, value); err != nil {
		t.Fatalf("failed to store the configuration: %v", err)
	}
}

func (f *fixture) username(t *testing.T) string {
	t.Helper()

	name := t.Name() + "-" + uuid.NewString()
	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := f.client.User.Delete().Where(usermodal.Username(name)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
	})

	return name
}

func TestStartupEndpointsRefuseOnceTheWizardIsCompleted(t *testing.T) {
	f := newFixture(t)
	f.markCompleted(t, true)

	ctx := context.Background()
	name := f.username(t)
	password := "hunter2"

	configurationResponse, err := f.server.GetStartupConfiguration(ctx, api.GetStartupConfigurationRequestObject{})
	if err != nil {
		t.Fatalf("GetStartupConfiguration: %v", err)
	}
	if _, ok := configurationResponse.(api.GetStartupConfiguration403Response); !ok {
		t.Errorf("GetStartupConfiguration answered %T, want 403", configurationResponse)
	}

	updateResponse, err := f.server.UpdateInitialConfiguration(ctx, api.UpdateInitialConfigurationRequestObject{
		JSONBody: &api.UpdateInitialConfigurationJSONRequestBody{UICulture: apiutil.Ptr("fr-FR")},
	})
	if err != nil {
		t.Fatalf("UpdateInitialConfiguration: %v", err)
	}
	if _, ok := updateResponse.(api.UpdateInitialConfiguration403Response); !ok {
		t.Errorf("UpdateInitialConfiguration answered %T, want 403", updateResponse)
	}

	firstUserResponse, err := f.server.GetFirstUser(ctx, api.GetFirstUserRequestObject{})
	if err != nil {
		t.Fatalf("GetFirstUser: %v", err)
	}
	if _, ok := firstUserResponse.(api.GetFirstUser403Response); !ok {
		t.Errorf("GetFirstUser answered %T, want 403", firstUserResponse)
	}

	firstUser2Response, err := f.server.GetFirstUser2(ctx, api.GetFirstUser2RequestObject{})
	if err != nil {
		t.Fatalf("GetFirstUser2: %v", err)
	}
	if _, ok := firstUser2Response.(api.GetFirstUser2403Response); !ok {
		t.Errorf("GetFirstUser2 answered %T, want 403", firstUser2Response)
	}

	userResponse, err := f.server.UpdateStartupUser(ctx, api.UpdateStartupUserRequestObject{
		JSONBody: &api.UpdateStartupUserJSONRequestBody{Name: &name, Password: &password},
	})
	if err != nil {
		t.Fatalf("UpdateStartupUser: %v", err)
	}
	if _, ok := userResponse.(api.UpdateStartupUser403Response); !ok {
		t.Errorf("UpdateStartupUser answered %T, want 403", userResponse)
	}

	remoteResponse, err := f.server.SetRemoteAccess(ctx, api.SetRemoteAccessRequestObject{
		JSONBody: &api.SetRemoteAccessJSONRequestBody{EnableRemoteAccess: true},
	})
	if err != nil {
		t.Fatalf("SetRemoteAccess: %v", err)
	}
	if _, ok := remoteResponse.(api.SetRemoteAccess403Response); !ok {
		t.Errorf("SetRemoteAccess answered %T, want 403", remoteResponse)
	}

	completeResponse, err := f.server.CompleteWizard(ctx, api.CompleteWizardRequestObject{})
	if err != nil {
		t.Fatalf("CompleteWizard: %v", err)
	}
	if _, ok := completeResponse.(api.CompleteWizard403Response); !ok {
		t.Errorf("CompleteWizard answered %T, want 403", completeResponse)
	}

	if exists, err := f.client.User.Query().Where(usermodal.Username(name)).Exist(ctx); err != nil {
		t.Fatalf("failed to query the user: %v", err)
	} else if exists {
		t.Error("UpdateStartupUser created a user after the wizard completed")
	}
}

func TestCompleteWizardClosesTheWizard(t *testing.T) {
	f := newFixture(t)
	f.markCompleted(t, false)

	ctx := context.Background()
	response, err := f.server.CompleteWizard(ctx, api.CompleteWizardRequestObject{})
	if err != nil {
		t.Fatalf("CompleteWizard: %v", err)
	}
	if _, ok := response.(api.CompleteWizard204Response); !ok {
		t.Fatalf("CompleteWizard answered %T, want 204", response)
	}

	completed, err := f.server.Completed(ctx)
	if err != nil {
		t.Fatalf("Completed: %v", err)
	}
	if !completed {
		t.Error("the wizard is still open after CompleteWizard")
	}

	configurationResponse, err := f.server.GetStartupConfiguration(ctx, api.GetStartupConfigurationRequestObject{})
	if err != nil {
		t.Fatalf("GetStartupConfiguration: %v", err)
	}
	if _, ok := configurationResponse.(api.GetStartupConfiguration403Response); !ok {
		t.Errorf("GetStartupConfiguration answered %T, want 403", configurationResponse)
	}
}

func TestUpdateInitialConfigurationStoresTheAnswers(t *testing.T) {
	f := newFixture(t)
	f.markCompleted(t, false)

	ctx := context.Background()
	response, err := f.server.UpdateInitialConfiguration(ctx, api.UpdateInitialConfigurationRequestObject{
		JSONBody: &api.UpdateInitialConfigurationJSONRequestBody{
			UICulture:                 apiutil.Ptr("fr-FR"),
			MetadataCountryCode:       apiutil.Ptr("FR"),
			PreferredMetadataLanguage: apiutil.Ptr("fr"),
		},
	})
	if err != nil {
		t.Fatalf("UpdateInitialConfiguration: %v", err)
	}
	if _, ok := response.(api.UpdateInitialConfiguration204Response); !ok {
		t.Fatalf("UpdateInitialConfiguration answered %T, want 204", response)
	}

	configurationResponse, err := f.server.GetStartupConfiguration(ctx, api.GetStartupConfigurationRequestObject{})
	if err != nil {
		t.Fatalf("GetStartupConfiguration: %v", err)
	}
	stored, ok := configurationResponse.(api.GetStartupConfiguration200JSONResponse)
	if !ok {
		t.Fatalf("GetStartupConfiguration answered %T, want 200", configurationResponse)
	}
	if got := apiutil.Deref(stored.UICulture); got != "fr-FR" {
		t.Errorf("UICulture = %q, want fr-FR", got)
	}
	if got := apiutil.Deref(stored.MetadataCountryCode); got != "FR" {
		t.Errorf("MetadataCountryCode = %q, want FR", got)
	}
	if got := apiutil.Deref(stored.PreferredMetadataLanguage); got != "fr" {
		t.Errorf("PreferredMetadataLanguage = %q, want fr", got)
	}
}

func TestUpdateStartupUserCreatesAnAdministrator(t *testing.T) {
	f := newFixture(t)
	f.markCompleted(t, false)

	ctx := context.Background()
	name := f.username(t)
	password := "hunter2"

	response, err := f.server.UpdateStartupUser(ctx, api.UpdateStartupUserRequestObject{
		JSONBody: &api.UpdateStartupUserJSONRequestBody{Name: &name, Password: &password},
	})
	if err != nil {
		t.Fatalf("UpdateStartupUser: %v", err)
	}
	if _, ok := response.(api.UpdateStartupUser204Response); !ok {
		t.Fatalf("UpdateStartupUser answered %T, want 204", response)
	}

	user, err := f.users.UserByUsername(ctx, name)
	if err != nil {
		t.Fatalf("failed to read the created user: %v", err)
	}
	if user.Edges.Policy == nil || !user.Edges.Policy.IsAdministrator {
		t.Error("the wizard created a user that is not an administrator")
	}
	if verified, err := auth.Verify(password, user.PasswordHash); err != nil || !verified {
		t.Errorf("the stored password does not verify: %v", err)
	}

	// The wizard can be walked back, so a second submission updates rather
	// than colliding with the account it just created.
	changed := "hunter3"
	response, err = f.server.UpdateStartupUser(ctx, api.UpdateStartupUserRequestObject{
		JSONBody: &api.UpdateStartupUserJSONRequestBody{Name: &name, Password: &changed},
	})
	if err != nil {
		t.Fatalf("UpdateStartupUser: %v", err)
	}
	if _, ok := response.(api.UpdateStartupUser204Response); !ok {
		t.Fatalf("UpdateStartupUser answered %T, want 204", response)
	}

	user, err = f.users.UserByUsername(ctx, name)
	if err != nil {
		t.Fatalf("failed to read the user: %v", err)
	}
	if verified, err := auth.Verify(changed, user.PasswordHash); err != nil || !verified {
		t.Errorf("the changed password does not verify: %v", err)
	}
}

func TestUpdateStartupUserKeepsTheWizardOpen(t *testing.T) {
	f := newFixture(t)
	f.markCompleted(t, false)

	ctx := context.Background()
	name := f.username(t)
	password := "hunter2"

	if _, err := f.server.UpdateStartupUser(ctx, api.UpdateStartupUserRequestObject{
		JSONBody: &api.UpdateStartupUserJSONRequestBody{Name: &name, Password: &password},
	}); err != nil {
		t.Fatalf("UpdateStartupUser: %v", err)
	}

	completed, err := f.server.Completed(ctx)
	if err != nil {
		t.Fatalf("Completed: %v", err)
	}
	if completed {
		t.Error("creating the first user closed the wizard before it finished")
	}
}

func TestSetRemoteAccessStoresTheNetworkConfiguration(t *testing.T) {
	f := newFixture(t)
	f.markCompleted(t, false)

	ctx := context.Background()
	response, err := f.server.SetRemoteAccess(ctx, api.SetRemoteAccessRequestObject{
		JSONBody: &api.SetRemoteAccessJSONRequestBody{
			EnableRemoteAccess:         true,
			EnableAutomaticPortMapping: true,
		},
	})
	if err != nil {
		t.Fatalf("SetRemoteAccess: %v", err)
	}
	if _, ok := response.(api.SetRemoteAccess204Response); !ok {
		t.Fatalf("SetRemoteAccess answered %T, want 204", response)
	}

	value, err := f.config.Configuration(ctx, networkConfigurationKey)
	if err != nil {
		t.Fatalf("failed to read the network configuration: %v", err)
	}

	var network networkConfiguration
	if err := json.Unmarshal(value, &network); err != nil {
		t.Fatalf("failed to decode the network configuration: %v", err)
	}
	if !network.EnableRemoteAccess || !network.EnableUPnP {
		t.Errorf("network configuration = %+v, want both enabled", network)
	}
}
