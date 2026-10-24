import { settingsSidebar } from "../settingsSidebar";

const refreshInterval = 3000;

const sessionStatusLabels = {
    starting: { label: "Starting", className: "" },
    logged_out: { label: "Logged out", className: "" },
    connecting: { label: "Connecting", className: "warning" },
    qr_pending: { label: "Waiting for QR scan", className: "warning" },
    qr_scanned: { label: "Confirm on phone", className: "warning" },
    logged_in: { label: "Logged in", className: "success" },
    disconnected: { label: "Disconnected", className: "danger" },
    error: { label: "Error", className: "danger" },
};

function generateSecret() {
    const bytes = new Uint8Array(24);
    crypto.getRandomValues(bytes);
    return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

export function pageZaloSettings() {
    app.store.title = "Zalo";

    const data = store({
        isLoading: true,
        isLoaded: false,
        isSaving: false,
        isRefreshing: false,
        busyAction: "",
        overview: null,
        form: {
            enabled: false,
            bridgeURL: "",
            // empty = keep the current secret
            secret: "",
        },
        // the last generated/typed secret (shown in the setup instructions)
        visibleSecret: "",
    });

    let timer = null;

    function initForm(overview) {
        data.form.enabled = !!overview.settings.enabled;
        data.form.bridgeURL = overview.settings.bridgeURL || "";
        data.form.secret = "";
    }

    async function loadOverview(initial = false) {
        if (!initial) {
            data.isRefreshing = true;
        }

        try {
            const overview = await app.pb.send("/api/zalo/overview", { requestKey: "zaloOverview" });
            data.overview = overview;
            if (initial) {
                initForm(overview);
                data.isLoaded = true;
            }
            data.isLoading = false;
        } catch (err) {
            if (!err.isAbort) {
                if (initial) {
                    app.checkApiError(err);
                }
                data.isLoading = false;
            }
        }

        data.isRefreshing = false;
    }

    function startPolling() {
        stopPolling();
        timer = setInterval(() => {
            if (!document.hidden && data.overview?.settings?.enabled) {
                loadOverview();
            }
        }, refreshInterval);
    }

    function stopPolling() {
        clearInterval(timer);
        timer = null;
    }

    async function saveSettings() {
        if (data.isSaving) {
            return;
        }

        data.isSaving = true;

        const body = {
            enabled: data.form.enabled,
            bridgeURL: data.form.bridgeURL,
        };
        if (data.form.secret) {
            body.secret = data.form.secret;
        }

        try {
            const overview = await app.pb.send("/api/zalo/settings", {
                method: "POST",
                body,
                requestKey: "zaloSettings",
            });
            data.overview = overview;
            if (data.form.secret) {
                data.visibleSecret = data.form.secret;
            }
            initForm(overview);
            app.store.errors = null;
            app.toasts.success("Successfully saved the Zalo settings.");
            // show/hide the Conversations menu
            app.store.loadConversationChannels();
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isSaving = false;
    }

    async function runAction(action, successMessage) {
        if (data.busyAction) {
            return;
        }

        data.busyAction = action;

        try {
            const result = await app.pb.send(`/api/zalo/${action}`, { method: "POST", requestKey: null });
            if (result?.error) {
                app.toasts.error(result.error);
            } else {
                app.toasts.success(successMessage);
            }
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        }

        data.busyAction = "";
        loadOverview();
        app.store.loadConversationChannels();
    }

    function confirmLogout() {
        app.modals.confirm(
            "Log out the Zalo account from the bridge? Incoming messages won't be received until you log in again.",
            () => runAction("logout", "Logged out."),
        );
    }

    const bridgeOnline = () => !!data.overview?.bridge?.online;
    const session = () => data.overview?.session || null;
    const sessionStatus = () => session()?.status || data.overview?.bridge?.state?.status || "";
    const isLoggedIn = () => sessionStatus() === "logged_in" || !!data.overview?.bridge?.state?.connected;

    function bridgeStatusView() {
        const overview = data.overview;
        if (!overview) {
            return null;
        }

        if (!overview.settings.enabled) {
            return t.div(
                { className: "zalo-status-row" },
                t.span({ className: "label" }, "Disabled"),
                t.span({ className: "txt-hint" }, "Enable the integration and save to check the bridge."),
            );
        }

        return t.div(
            { className: "zalo-status-row" },
            bridgeOnline()
                ? t.span(
                    { className: "label success" },
                    t.i({ className: "ri-checkbox-circle-line", ariaHidden: true }),
                    " Online",
                )
                : t.span(
                    { className: "label danger" },
                    t.i({ className: "ri-error-warning-line", ariaHidden: true }),
                    " Offline",
                ),
            t.code({ className: "txt-sm txt-hint" }, overview.settings.bridgeURL),
            bridgeOnline() && overview.bridge.latencyMs !== undefined
                ? t.span({ className: "txt-hint txt-sm" }, `${overview.bridge.latencyMs} ms`)
                : null,
            overview.bridge.error
                ? t.div({ className: "txt-danger txt-sm zalo-status-error" }, overview.bridge.error)
                : null,
        );
    }

    function accountView() {
        const s = session();
        const state = data.overview.bridge?.state || {};
        const status = sessionStatus();
        const statusInfo = sessionStatusLabels[status] || { label: status || "Not logged in", className: "" };

        const accountName = s?.account_name || state.accountName || "";
        const accountId = s?.account_id || state.accountId || "";
        const message = s?.message || state.message || "";

        return t.div(
            { className: "zalo-account" },
            t.div(
                { className: "zalo-status-row" },
                t.span({ className: `label ${statusInfo.className}` }, statusInfo.label),
                accountName ? t.strong(null, accountName) : null,
                accountId ? t.code({ className: "txt-hint txt-sm" }, accountId) : null,
            ),
            message ? t.div({ className: "txt-hint txt-sm m-t-5" }, message) : null,
            !bridgeOnline()
                ? t.div({ className: "txt-hint txt-sm m-t-5" }, "Start the bridge to log in, log out or send messages.")
                : null,
            status === "qr_pending" && s?.qr_image
                ? t.div(
                    { className: "zalo-qr" },
                    t.img({ src: s.qr_image, alt: "Zalo login QR code" }),
                    t.div(
                        { className: "txt-sm txt-hint" },
                        "Open the Zalo app on your phone and scan this QR code. It expires after about 100 seconds and is renewed automatically.",
                    ),
                )
                : null,
            t.div(
                { className: "flex gap-10 flex-wrap m-t-sm" },
                !isLoggedIn()
                    ? t.button(
                        {
                            type: "button",
                            className: () => `btn ${data.busyAction === "login" ? "loading" : ""}`,
                            disabled: () =>
                                !bridgeOnline() || !!data.busyAction || status === "qr_pending"
                                || status === "qr_scanned",
                            onclick: () => runAction("login", "Generating the QR code..."),
                        },
                        t.i({ className: "ri-qr-code-line", ariaHidden: true }),
                        t.span({ className: "txt" }, status === "qr_pending" ? "Waiting for scan" : "Login with QR"),
                    )
                    : null,
                isLoggedIn() || status === "disconnected"
                    ? t.button(
                        {
                            type: "button",
                            className: () => `btn secondary ${data.busyAction === "logout" ? "loading" : ""}`,
                            disabled: () => !bridgeOnline() || !!data.busyAction,
                            onclick: confirmLogout,
                        },
                        t.i({ className: "ri-logout-box-r-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "Logout"),
                    )
                    : null,
                t.button(
                    {
                        type: "button",
                        className: () => `btn transparent secondary ${data.busyAction === "kick" ? "loading" : ""}`,
                        disabled: () => !bridgeOnline() || !isLoggedIn() || !!data.busyAction,
                        onclick: () => runAction("kick", "Sending the pending messages."),
                    },
                    t.i({ className: "ri-send-plane-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Send pending messages"),
                ),
            ),
        );
    }

    function setupView() {
        const secret = data.visibleSecret || "<the bridge secret>";
        const env = [
            "PB_URL=http://127.0.0.1:8090",
            "PB_SUPERUSER_EMAIL=bridge@example.com",
            "PB_SUPERUSER_PASSWORD=\"<password>\"",
            "BRIDGE_HOST=127.0.0.1",
            "BRIDGE_PORT=" + ((data.overview?.settings?.bridgeURL || "").match(/:(\d+)/)?.[1] || "3939"),
            "BRIDGE_SECRET=" + secret,
            "ZALO_SELF_LISTEN=true",
            "ZALO_AUTO_QR_LOGIN=true",
            "ZALO_SEND_DELAY_MS=1500",
        ].join("\n");

        return t.details(
            { className: "zalo-setup m-t-base" },
            t.summary({ className: "txt-bold" }, "Bridge setup"),
            t.ol(
                { className: "txt-sm" },
                t.li(null, "Create a dedicated superuser for the bridge (Settings > Superusers)."),
                t.li(
                    null,
                    "Configure the bridge ",
                    t.code(null, ".env"),
                    " (BRIDGE_SECRET must match the secret above, PB_URL is the address of this app as seen from the bridge):",
                ),
            ),
            app.components.codeBlock({ language: "bash", value: env }),
            t.ol(
                { className: "txt-sm", start: 3 },
                t.li(
                    null,
                    "Start the bridge (e.g. the zalo-bridge systemd service) and check that it shows Online above.",
                ),
                t.li(
                    null,
                    "If you used the old PocketBase hooks, delete ",
                    t.code(null, "pb_hooks/zalo.pb.js"),
                    " and ",
                    t.code(null, "pb_hooks/zalo/bridge.js"),
                    " - they are now built in. The existing collections and messages are kept.",
                ),
            ),
        );
    }

    return t.div(
        {
            pbEvent: "pageZaloSettings",
            className: "page page-zalo-settings",
            onmount: () => {
                loadOverview(true);
                startPolling();
            },
            onunmount: () => stopPolling(),
        },
        settingsSidebar(),
        t.div(
            { className: "page-content full-height" },
            t.header(
                { className: "page-header" },
                t.nav(
                    { className: "breadcrumbs" },
                    t.div({ className: "breadcrumb-item" }, "Settings"),
                    t.div({ className: "breadcrumb-item" }, () => app.store.title),
                ),
            ),
            t.div(
                { className: "wrapper m-b-base" },
                () => {
                    if (data.isLoading) {
                        return t.div({ className: "block txt-center" }, t.span({ className: "loader lg" }));
                    }

                    // note: only the loaded flag is read here so that the polling
                    // refreshes don't re-create the form inputs while typing
                    if (!data.isLoaded) {
                        return t.div({ className: "txt-hint" }, "Failed to load the Zalo settings.");
                    }

                    return t.div(
                        null,
                        t.p(
                            { className: "txt-hint m-b-base" },
                            "Connect a personal Zalo account through the zalo-bridge process. Incoming messages are saved in ",
                            t.code(null, "zalo_messages"),
                            " and records created in ",
                            t.code(null, "zalo_outbox"),
                            " are sent by the bridge.",
                        ),
                        t.form(
                            {
                                className: "zalo-settings-form grid",
                                inert: () => data.isSaving,
                                onsubmit: (e) => {
                                    e.preventDefault();
                                    saveSettings();
                                },
                            },
                            t.div(
                                { className: "col-lg-12" },
                                t.div(
                                    { className: "field" },
                                    t.input({
                                        id: "zalo_enabled",
                                        name: "enabled",
                                        type: "checkbox",
                                        className: "switch",
                                        checked: () => data.form.enabled,
                                        onchange: (e) => (data.form.enabled = e.target.checked),
                                    }),
                                    t.label({ htmlFor: "zalo_enabled" }, "Enable Zalo"),
                                ),
                            ),
                            t.div(
                                { className: "col-lg-6" },
                                t.div(
                                    { className: "field" },
                                    t.label({ htmlFor: "zalo_bridgeURL" }, "Bridge URL"),
                                    t.input({
                                        id: "zalo_bridgeURL",
                                        name: "bridgeURL",
                                        type: "url",
                                        placeholder: "http://127.0.0.1:3939",
                                        value: () => data.form.bridgeURL,
                                        oninput: (e) => (data.form.bridgeURL = e.target.value),
                                    }),
                                ),
                                t.div(
                                    { className: "field-help" },
                                    "Internal address of the bridge. Never expose the bridge port to the Internet.",
                                ),
                            ),
                            t.div(
                                { className: "col-lg-6" },
                                t.div(
                                    { className: "field" },
                                    t.label({ htmlFor: "zalo_secret" }, "Bridge secret"),
                                    t.input({
                                        id: "zalo_secret",
                                        name: "secret",
                                        type: "password",
                                        autocomplete: "new-password",
                                        placeholder: () =>
                                            data.overview?.settings?.hasSecret
                                                ? "* * * * * * (unchanged)"
                                                : "At least 16 characters",
                                        value: () => data.form.secret,
                                        oninput: (e) => (data.form.secret = e.target.value),
                                    }),
                                ),
                                t.div(
                                    { className: "field-help" },
                                    "The BRIDGE_SECRET of the bridge (sent as X-Bridge-Secret). ",
                                    t.button(
                                        {
                                            type: "button",
                                            className: "link-primary",
                                            onclick: () => {
                                                data.form.secret = generateSecret();
                                                data.visibleSecret = data.form.secret;
                                            },
                                        },
                                        "Generate a new secret",
                                    ),
                                ),
                            ),
                            t.div(
                                { className: "col-lg-12 flex" },
                                t.button(
                                    {
                                        type: "submit",
                                        className: () => `btn m-l-auto ${data.isSaving ? "loading" : ""}`,
                                        disabled: () => data.isSaving,
                                    },
                                    t.span({ className: "txt" }, "Save changes"),
                                ),
                            ),
                        ),
                        t.div(
                            { className: "zalo-card m-t-base" },
                            t.div(
                                { className: "flex gap-10 m-b-sm" },
                                t.div({ className: "txt-lg" }, "Bridge"),
                                app.components.refreshButton({
                                    className: "btn sm transparent secondary circle",
                                    tooltip: "Check bridge",
                                    onclick: () => loadOverview(),
                                }),
                            ),
                            () => bridgeStatusView(),
                        ),
                        () =>
                            data.overview?.settings?.enabled
                                ? t.div(
                                    { className: "zalo-card m-t-base" },
                                    t.div({ className: "txt-lg m-b-sm" }, "Zalo account"),
                                    () => accountView(),
                                )
                                : null,
                        () => setupView(),
                    );
                },
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
