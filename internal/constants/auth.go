package constants

// AuthInvalidCredentials is the single message returned for every login
// failure mode so a caller cannot distinguish "no such user" from "wrong
// password" (AUTH-01 acceptance: no user enumeration).
const AuthInvalidCredentials = "invalid credentials"

// AuthInvalidRefreshToken covers not-found, revoked, and expired refresh
// tokens alike — the client sees the same 401 message in every case.
const AuthInvalidRefreshToken = "invalid refresh token"
