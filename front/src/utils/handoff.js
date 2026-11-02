/**
 * StackBlaze handoff context. Users minted by the embedding product's
 * handoff (api/handoff.go) carry a synthetic @handoff.local email, and the
 * consume step stores where the product is (coroot_return_url cookie).
 */
export function isHandoffUser(api) {
    const email = (api && api.auth && api.auth.user && api.auth.user.email) || '';
    return email.endsWith('@handoff.local');
}

/** The embedding product's origin (https://dashboard…), or '' when unknown. */
export function returnUrlOrigin() {
    const m = document.cookie.match(/(?:^|; )coroot_return_url=([^;]*)(?:;|$)/);
    if (!m) {
        return '';
    }
    try {
        return new URL(decodeURIComponent(m[1])).origin;
    } catch {
        return '';
    }
}

/** The platform's OpenTelemetry collector, reachable from every app namespace. */
export const PLATFORM_OTLP_ENDPOINT = 'http://otel-collector.otel.svc:4318';
