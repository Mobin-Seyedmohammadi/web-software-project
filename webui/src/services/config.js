// The frontend is served from the same host the backend runs on, just on a
// different port. Building the API URL from window.location at runtime
// (instead of the __API_URL__ build-time constant, which is hardcoded to
// localhost for evaluation) lets the app work when opened from another
// device on the network, e.g. http://192.168.1.45:8080 -> http://192.168.1.45:3000.
export const API_URL = `${window.location.protocol}//${window.location.hostname}:3000`
