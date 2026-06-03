import { settingsSidebar } from "../settingsSidebar";

const secretPlaceholder = "* * * * * *";

export function pageCredentialsSettings() {
    app.store.title = "Credentials";

    const data = store({
        isLoading: false,
        isSaving: false,
        isTestingTelegram: false,
        isTestingGoogleSheets: false,
        formSettings: null,
        initSerialized: "null",
        get hasChanges() {
            return data.initSerialized != JSON.stringify(data.formSettings);
        },
    });

    loadSettings();

    async function loadSettings() {
        data.isLoading = true;

        try {
            const settings = await app.pb.settings.getAll();
            init(settings);

            data.isLoading = false;
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
                // data.isLoading = false; don't reset in case of a server error
            }
        }
    }

    async function save() {
        if (data.isSaving || !data.hasChanges) {
            return;
        }

        data.isSaving = true;

        try {
            const redacted = app.utils.filterRedactedProps(data.formSettings);
            const settings = await app.pb.settings.update(redacted);
            init(settings);

            app.toasts.success("Successfully saved credentials.");
        } catch (err) {
            app.checkApiError(err);
        }

        data.isSaving = false;
    }

    async function testTelegram() {
        if (data.isTestingTelegram) {
            return;
        }

        data.isTestingTelegram = true;

        try {
            await app.pb.send("/api/settings/test/telegram", {
                method: "POST",
                body: app.utils.filterRedactedProps(data.formSettings.credentials.telegram),
            });

            app.toasts.success("Telegram credentials verified.");
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isTestingTelegram = false;
    }

    async function testGoogleSheets() {
        if (data.isTestingGoogleSheets) {
            return;
        }

        data.isTestingGoogleSheets = true;

        try {
            await app.pb.send("/api/settings/test/google-sheets", {
                method: "POST",
                body: app.utils.filterRedactedProps(data.formSettings.credentials.googleSheets),
            });

            app.toasts.success("Google Sheets credentials verified.");
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isTestingGoogleSheets = false;
    }

    function init(settings = {}) {
        // refresh local app settings
        app.store.settings = JSON.parse(JSON.stringify(settings));

        const credentials = settings?.credentials || {};
        const telegram = credentials.telegram || {};
        if (!telegram.baseURL) {
            telegram.baseURL = "https://api.telegram.org";
        }

        data.formSettings = {
            credentials: {
                telegram: telegram,
                googleSheets: credentials.googleSheets || {},
            },
        };

        data.initSerialized = JSON.stringify(data.formSettings);
    }

    function reset() {
        data.formSettings = JSON.parse(data.initSerialized);
    }

    function setMissingSecret(config, prop) {
        if (typeof config[prop] === "undefined") {
            config[prop] = "";
        }
    }

    function secretInput(config, prop, attrs = {}) {
        return t.input({
            ...attrs,
            type: "password",
            autocomplete: "new-password",
            value: () => config[prop] || "",
            oninput: (e) => config[prop] = e.target.value,
            onkeyup: (e) => {
                if (e.key == "Backspace") {
                    setMissingSecret(config, prop);
                }
            },
            placeholder: () => typeof config[prop] !== "undefined" ? "" : secretPlaceholder,
        });
    }

    return t.div(
        { pbEvent: "pageCredentialsSettings", className: "page page-credentials-settings" },
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

                    const telegram = data.formSettings.credentials.telegram;
                    const googleSheets = data.formSettings.credentials.googleSheets;

                    return t.form(
                        {
                            pbEvent: "credentialsSettingsForm",
                            className: "grid credentials-settings-form",
                            inert: () => data.isSaving,
                            onsubmit: (e) => {
                                e.preventDefault();
                                save();
                            },
                        },
                        t.div(
                            { className: "col-lg-12 txt-lg" },
                            t.p(null, "Manage reusable third-party credentials."),
                        ),
                        t.div(
                            { className: "col-lg-12" },
                            t.h5(null, "Telegram"),
                        ),
                        t.div(
                            { className: "col-lg-12" },
                            t.div(
                                { className: "field" },
                                t.input({
                                    id: "credentials.telegram.enabled",
                                    name: "credentials.telegram.enabled",
                                    type: "checkbox",
                                    className: "switch",
                                    checked: () => !!telegram.enabled,
                                    onchange: (e) => telegram.enabled = e.target.checked,
                                }),
                                t.label(
                                    { htmlFor: "credentials.telegram.enabled" },
                                    t.span({ className: "txt" }, "Enable Telegram credentials"),
                                ),
                            ),
                        ),
                        t.div(
                            { className: "col-lg-12" },
                            app.components.slide(
                                () => telegram.enabled,
                                t.div(
                                    { className: "grid" },
                                    t.div(
                                        { className: "col-lg-6" },
                                        t.div(
                                            { className: "field" },
                                            t.label({ htmlFor: "credentials.telegram.baseURL" }, "Base URL"),
                                            t.input({
                                                id: "credentials.telegram.baseURL",
                                                name: "credentials.telegram.baseURL",
                                                type: "url",
                                                required: () => telegram.enabled,
                                                value: () => telegram.baseURL || "",
                                                oninput: (e) => telegram.baseURL = e.target.value,
                                            }),
                                        ),
                                    ),
                                    t.div(
                                        { className: "col-lg-6" },
                                        t.div(
                                            { className: "field" },
                                            t.label({ htmlFor: "credentials.telegram.accessToken" }, "Access token"),
                                            secretInput(telegram, "accessToken", {
                                                id: "credentials.telegram.accessToken",
                                                name: "credentials.telegram.accessToken",
                                                required: () =>
                                                    telegram.enabled && typeof telegram.accessToken !== "undefined",
                                            }),
                                        ),
                                    ),
                                ),
                                t.div(
                                    { className: "col-lg-12" },
                                    t.button(
                                        {
                                            type: "button",
                                            className: () =>
                                                `btn outline expanded-sm ${data.isTestingTelegram ? "loading" : ""}`,
                                            disabled: () => data.isTestingTelegram || data.isSaving,
                                            onclick: testTelegram,
                                        },
                                        t.i({ className: "ri-plug-line", ariaHidden: true }),
                                        t.span({ className: "txt" }, "Test Telegram"),
                                    ),
                                ),
                            ),
                        ),
                        t.div({ className: "col-lg-12" }, t.hr()),
                        t.div(
                            { className: "col-lg-12" },
                            t.h5(null, "Google Sheets"),
                        ),
                        t.div(
                            { className: "col-lg-12" },
                            t.div(
                                { className: "field" },
                                t.input({
                                    id: "credentials.googleSheets.enabled",
                                    name: "credentials.googleSheets.enabled",
                                    type: "checkbox",
                                    className: "switch",
                                    checked: () => !!googleSheets.enabled,
                                    onchange: (e) => googleSheets.enabled = e.target.checked,
                                }),
                                t.label(
                                    { htmlFor: "credentials.googleSheets.enabled" },
                                    t.span({ className: "txt" }, "Enable Google Sheets credentials"),
                                ),
                            ),
                        ),
                        t.div(
                            { className: "col-lg-12" },
                            app.components.slide(
                                () => googleSheets.enabled,
                                t.div(
                                    { className: "grid" },
                                    t.div(
                                        { className: "col-lg-6" },
                                        t.div(
                                            { className: "field" },
                                            t.label(
                                                { htmlFor: "credentials.googleSheets.oauthRedirectURL" },
                                                "OAuth Redirect URL",
                                            ),
                                            t.input({
                                                id: "credentials.googleSheets.oauthRedirectURL",
                                                name: "credentials.googleSheets.oauthRedirectURL",
                                                type: "url",
                                                required: () => googleSheets.enabled,
                                                value: () => googleSheets.oauthRedirectURL || "",
                                                oninput: (e) => googleSheets.oauthRedirectURL = e.target.value,
                                            }),
                                        ),
                                    ),
                                    t.div(
                                        { className: "col-lg-6" },
                                        t.div(
                                            { className: "field" },
                                            t.label(
                                                { htmlFor: "credentials.googleSheets.clientID" },
                                                "Client ID",
                                            ),
                                            t.input({
                                                id: "credentials.googleSheets.clientID",
                                                name: "credentials.googleSheets.clientID",
                                                type: "text",
                                                required: () => googleSheets.enabled,
                                                value: () => googleSheets.clientID || "",
                                                oninput: (e) => googleSheets.clientID = e.target.value,
                                            }),
                                        ),
                                    ),
                                    t.div(
                                        { className: "col-lg-6" },
                                        t.div(
                                            { className: "field" },
                                            t.label(
                                                { htmlFor: "credentials.googleSheets.clientSecret" },
                                                "Client Secret",
                                            ),
                                            secretInput(googleSheets, "clientSecret", {
                                                id: "credentials.googleSheets.clientSecret",
                                                name: "credentials.googleSheets.clientSecret",
                                                required: () =>
                                                    googleSheets.enabled
                                                    && typeof googleSheets.clientSecret !== "undefined",
                                            }),
                                        ),
                                    ),
                                ),
                                t.div(
                                    { className: "col-lg-12" },
                                    t.button(
                                        {
                                            type: "button",
                                            className: () =>
                                                `btn outline expanded-sm ${
                                                    data.isTestingGoogleSheets ? "loading" : ""
                                                }`,
                                            disabled: () => data.isTestingGoogleSheets || data.isSaving,
                                            onclick: testGoogleSheets,
                                        },
                                        t.i({ className: "ri-plug-line", ariaHidden: true }),
                                        t.span({ className: "txt" }, "Test Google Sheets"),
                                    ),
                                ),
                            ),
                        ),
                        t.div({ className: "col-lg-12" }, t.hr()),
                        t.div(
                            { className: "col-lg-12" },
                            t.div(
                                { className: "flex" },
                                t.div({ className: "m-r-auto" }),
                                t.button(
                                    {
                                        hidden: () => !data.hasChanges,
                                        type: "button",
                                        className: "btn transparent secondary",
                                        onclick: reset,
                                    },
                                    t.span({ className: "txt" }, "Cancel"),
                                ),
                                t.button(
                                    {
                                        className: () => `btn expanded-lg ${data.isSaving ? "loading" : ""}`,
                                        disabled: () => !data.hasChanges || data.isSaving,
                                    },
                                    t.span({ className: "txt" }, "Save changes"),
                                ),
                            ),
                        ),
                    );
                },
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
