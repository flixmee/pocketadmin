export function localesList(propsArg = {}) {
    const props = store({
        reset: null,
    });

    const watchers = app.utils.extendStore(props, propsArg);

    const data = store({
        isLoading: false,
        isSaving: false,
        isDeleting: {},
        locales: [],
        form: emptyForm(),
    });

    async function loadLocales() {
        data.isLoading = true;

        try {
            data.locales = await app.pb.send("/api/locales", {
                requestKey: "localesList.load",
            });
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isLoading = false;
    }

    async function saveLocale(locale = null) {
        if (data.isSaving) {
            return;
        }

        data.isSaving = true;

        const payload = locale
            ? {
                code: locale.code,
                name: locale.label || locale.name,
                enabled: locale.enabled,
                is_default: locale.is_default,
            }
            : data.form;

        try {
            if (locale?.id) {
                await app.pb.send(`/api/locales/${locale.id}`, {
                    method: "PATCH",
                    body: payload,
                });
                app.toasts.success("Locale updated.");
            } else {
                await app.pb.send("/api/locales", {
                    method: "POST",
                    body: payload,
                });
                data.form = emptyForm();
                app.toasts.success("Locale created.");
            }

            app.store.errors = null;
            await loadLocales();
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err, false);
                app.toasts.error(err.message || "Failed to save locale.", { key: "localeSave" });
            }
        }

        data.isSaving = false;
    }

    async function deleteLocale(locale) {
        if (!locale?.id || data.isDeleting[locale.id]) {
            return;
        }

        data.isDeleting[locale.id] = true;

        try {
            await app.pb.send(`/api/locales/${locale.id}`, {
                method: "DELETE",
            });
            app.toasts.success("Locale deleted.");
            await loadLocales();
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        delete data.isDeleting[locale.id];
    }

    function confirmDelete(locale) {
        app.modals.confirm(
            `Do you really want to delete locale "${locale.code}"?`,
            () => deleteLocale(locale),
            null,
            { yesButton: "Delete", noButton: "Cancel" },
        );
    }

    function setDefault(locale) {
        locale.is_default = true;
        locale.enabled = true;
        saveLocale(locale);
    }

    return t.div(
        {
            pbEvent: "localesList",
            className: "list locales-list",
            onmount: () => {
                loadLocales();
                watchers.push(watch(() => props.reset, () => loadLocales()));
            },
            onunmount: () => watchers.forEach((w) => w?.unwatch()),
        },
        t.div(
            { className: "list-content" },
            t.div(
                {
                    hidden: () => !data.isLoading || data.locales.length,
                    className: "list-item",
                },
                t.div({ className: "skeleton-loader" }),
            ),
            () =>
                data.locales.map((locale) => {
                    return t.div(
                        { className: () => `list-item ${data.isLoading ? "faded" : ""}` },
                        t.i({
                            className: () => `ri-translate-2 ${locale.enabled ? "txt-success" : "txt-hint"}`,
                            ariaHidden: true,
                        }),
                        t.div(
                            { className: "content block" },
                            t.div(
                                { className: "flex flex-wrap gap-5" },
                                t.span({ className: "txt-bold txt-code" }, () => locale.code),
                                t.span({ className: () => `label ${locale.enabled ? "success" : ""}` }, () =>
                                    locale.enabled ? "Enabled" : "Disabled"),
                                t.span(
                                    {
                                        hidden: () =>
                                            !locale.is_default,
                                        className: "label primary",
                                    },
                                    "Default",
                                ),
                            ),
                            t.div({ className: "txt-sm txt-hint m-t-5" }, () => locale.label || locale.name),
                        ),
                        t.nav(
                            { className: "actions autohide" },
                            t.button(
                                {
                                    type: "button",
                                    ariaLabel: app.attrs.tooltip(locale.enabled ? "Disable" : "Enable"),
                                    className: "btn sm circle secondary transparent",
                                    disabled: () => data.isSaving || locale.is_default,
                                    onclick: () => {
                                        locale.enabled = !locale.enabled;
                                        saveLocale(locale);
                                    },
                                },
                                t.i({
                                    className: () => locale.enabled ? "ri-pause-line" : "ri-play-line",
                                    ariaHidden: true,
                                }),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    ariaLabel: app.attrs.tooltip("Make default"),
                                    className: "btn sm circle secondary transparent",
                                    disabled: () => data.isSaving || locale.is_default || !locale.enabled,
                                    onclick: () => setDefault(locale),
                                },
                                t.i({ className: "ri-star-line", ariaHidden: true }),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    ariaLabel: app.attrs.tooltip("Delete"),
                                    className: () =>
                                        `btn sm circle secondary transparent ${
                                            data.isDeleting[locale.id] ? "loading" : ""
                                        }`,
                                    disabled: () => data.isSaving || locale.is_default,
                                    onclick: () => confirmDelete(locale),
                                },
                                t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                            ),
                        ),
                    );
                }),
            t.div(
                { className: "list-item" },
                t.div(
                    { className: "content block" },
                    t.div(
                        { className: "grid sm" },
                        t.div(
                            { className: "col-3" },
                            t.input({
                                type: "text",
                                placeholder: "Code, e.g. vi",
                                value: () => data.form.code,
                                oninput: (e) => data.form.code = e.target.value,
                            }),
                        ),
                        t.div(
                            { className: "col-5" },
                            t.input({
                                type: "text",
                                placeholder: "Name",
                                value: () => data.form.name,
                                oninput: (e) => data.form.name = e.target.value,
                            }),
                        ),
                        t.label(
                            { className: "col-2 checkbox" },
                            t.input({
                                type: "checkbox",
                                checked: () => data.form.enabled,
                                onchange: (e) => data.form.enabled = e.target.checked,
                            }),
                            t.span({ className: "txt" }, "Enabled"),
                        ),
                        t.label(
                            { className: "col-2 checkbox" },
                            t.input({
                                type: "checkbox",
                                checked: () => data.form.is_default,
                                onchange: (e) => {
                                    data.form.is_default = e.target.checked;
                                    if (data.form.is_default) {
                                        data.form.enabled = true;
                                    }
                                },
                            }),
                            t.span({ className: "txt" }, "Default"),
                        ),
                    ),
                ),
                t.button(
                    {
                        type: "button",
                        className: () => `btn secondary ${data.isSaving ? "loading" : ""}`,
                        disabled: () => data.isSaving || !data.form.code || !data.form.name,
                        onclick: () => saveLocale(),
                    },
                    t.i({ className: "ri-add-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Add locale"),
                ),
            ),
        ),
    );
}

function emptyForm() {
    return {
        code: "",
        name: "",
        enabled: true,
        is_default: false,
    };
}
