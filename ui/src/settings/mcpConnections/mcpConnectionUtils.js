export const mcpConnectionKindOptions = [
    { value: "http", label: "MCP server (Streamable HTTP)" },
    { value: "gmail", label: "Gmail (built-in)" },
];

export function googleOAuthRedirectURL() {
    const appURL = (app.store.settings?.meta?.appURL || window.location.origin).replace(/\/+$/, "");
    return appURL + "/api/mcp/oauth/google/callback";
}

/**
 * Starts the Google sign-in flow of a Gmail connection in a popup window.
 *
 * @param {Object} server The Gmail connection.
 * @param {Object} [options]
 * @param {Window} [options.popup] Already opened popup (to avoid popup blockers after async calls).
 * @param {Function} [options.onconnect] Called after the flow completes (successfully or not).
 */
export async function connectGoogleAccount(server, options = {}) {
    const popup = options.popup || window.open("", "pa_google_oauth", "width=520,height=700");

    let result;
    try {
        result = await app.pb.send(`/api/mcp/servers/${encodeURIComponent(server.id)}/oauth/start`, {
            method: "POST",
            requestKey: null,
        });
    } catch (err) {
        popup?.close();
        if (!err.isAbort) {
            app.checkApiError(err);
        }
        return;
    }

    if (!popup || popup.closed) {
        // popup blocked - continue in the current tab
        window.location.href = result.authURL;
        return;
    }

    popup.location.href = result.authURL;

    let done = false;

    function finish() {
        if (done) {
            return;
        }
        done = true;
        window.removeEventListener("message", onMessage);
        clearInterval(closedWatcher);
        options.onconnect?.();
    }

    function onMessage(e) {
        if (e.data?.type !== "pocketadmin:mcp-oauth") {
            return;
        }

        if (e.data.success) {
            app.toasts.success(e.data.message || "Account connected.");
        } else {
            app.toasts.error(e.data.message || "Failed to connect the account.");
        }

        finish();
    }

    window.addEventListener("message", onMessage);

    // the window could be closed without completing the flow
    const closedWatcher = setInterval(() => {
        if (popup.closed) {
            finish();
        }
    }, 800);
}
