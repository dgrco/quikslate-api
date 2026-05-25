# Authflow Backend

NOTE: This will constantly be updated. Expect it to be unfinished.

## Technical Decisions

### Authentication: JWT Authentication w/ Refresh Token Rotation
JSON Web Token (JWT) authentication is a common alternative to session-based authentication.

Each of these two types of authentication have their own pros and cons.
For example, session-based tokens can be revoked at any time, causing anyone with the session to be unauthenticated (logged out).
JWTs, on the other hand, do not have this ability. However, they have their own benefits like not having to query the DB on each request.

#### Access Tokens
An access token is the thing the server uses to authenticate you. More specifically, it validates that your token is legitimate
(meaning the server signed it, and it hasn't been tampered with) and that it has not yet expired.

In this app, the `jwt` package does the heavy lifting for access tokens, as trying to implement JWT signing/parsing logic adds unnecessary security risk.
Ultimately, these are generated with registered claims and the user's ID, and are sent to the client.

The client will need to determine how these are stored. In this case, the client will store the access token in-memory for maximum security.

#### Refresh Tokens

