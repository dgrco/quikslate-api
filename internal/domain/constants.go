package domain

const (
	MAX_BUSINESS_NAME_LEN = 256
	MAX_LOCATION_NAME_LEN = 256
	MAX_LOCATION_ADDR_LEN = 256
	MAX_POSITION_NAME_LEN = 256
	MAX_USER_NAME_LEN     = 256
	MAX_USER_EMAIL_LEN    = 254 // RFC 5321 path limit minus the angle brackets
	// bcrypt silently ignores input past 72 bytes, so anything longer would
	// only look stronger than it is.
	MAX_USER_PASSWORD_LEN = 72
	MIN_USER_PASSWORD_LEN = 8
)
