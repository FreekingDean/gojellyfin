# Single sign-on with OpenID Connect

gojellyfin can let users sign in through an OpenID Connect provider. When configured, the login page offers a **Sign in with SSO** option.

## What you need

- The provider's issuer URL.
- A client ID.
- A public hostname and scheme for gojellyfin.
- A client secret if your provider registered a confidential client.

Register this redirect URI with your provider:

```text
https://media.example.com/sso/oidc/callback
```

Replace the example hostname and scheme with the values your deployment actually uses.

## Helm settings

At minimum, configure:

```yaml
hostname: media.example.com
scheme: https
oidc:
  issuerUrl: https://identity.example.com
  clientId: gojellyfin
```

The feature remains off until both `issuerUrl` and `clientId` are set. If your client has a secret, store it in a Kubernetes Secret and set `oidc.existingSecret`; the key defaults to `OIDC_CLIENT_SECRET`.

Other useful settings:

| Value | What it controls |
|---|---|
| `oidc.scopes` | Space-separated scopes. Empty uses `openid profile email`. |
| `oidc.usernameClaim` | Claim used for the username. Empty uses `preferred_username`. |
| `oidc.groupsClaim` | Claim used for group membership. Empty uses `groups`. |
| `oidc.adminGroup` | Group whose members sign in as administrators. |
| `oidc.allowedGroup` | If set, only members of this group may sign in. |
| `oidc.linkExistingUsers` | Allows first sign-in to claim a local account with the same username. |

Be careful with account linking. Only enable `linkExistingUsers` when users cannot choose or change their identity-provider username themselves.
