# First login

A fresh gojellyfin database starts with no user accounts. Create the first administrator from the server container after the database schema has been applied.

For a Helm deployment:

```sh
kubectl exec -i deploy/gojellyfin -- gojellyfin adduser Dean
```

Replace `Dean` with the username you want. The command prompts for the password; avoid putting passwords directly into shell history.

If your release uses a different name, adjust the Deployment name in the command.

## Sign in

Open the address you configured for gojellyfin and sign in with the account you just created. The web client is served by the `web` workload when it is enabled.

If you are migrating from Jellyfin, use the [migration guide](../migration/from-jellyfin.md) before asking everyone to sign in again. The migration tool can carry users, password hashes and device tokens across.

## Can't sign in?

- Confirm the API pods are ready and can reach PostgreSQL.
- Check that the hostname and scheme advertised by the server match the address your browser uses.
- If you enabled OpenID Connect, check the provider's redirect URI and client settings in [Single sign-on](../configuration/single-sign-on.md).
- If the account exists but its password is unknown, an administrator can use `gojellyfin resetpassword` from the server container.
