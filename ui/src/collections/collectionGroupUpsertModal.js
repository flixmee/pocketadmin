window.app = window.app || {};
window.app.modals = window.app.modals || {};

window.app.modals.openCollectionGroupUpsert = function(settings = {
    initialName: "",
    title: "",
    submitLabel: "",
    onsubmit: null,
}) {
    const modal = collectionGroupUpsertModal(settings || {});

    document.body.appendChild(modal);

    app.modals.open(modal);
};

function collectionGroupUpsertModal(settings) {
    let modal;

    const uniqueId = "collection_group_upsert_" + app.utils.randomString();

    const data = store({
        name: settings.initialName || "",
        isSubmitting: false,
        get normalizedName() {
            return app.utils.normalizeCollectionGroup(data.name);
        },
        get canSubmit() {
            return !data.isSubmitting && !!data.normalizedName;
        },
    });

    async function submit() {
        if (!data.canSubmit) {
            return;
        }

        data.isSubmitting = true;

        try {
            const result = await settings.onsubmit?.(data.normalizedName);
            if (result === false) {
                data.isSubmitting = false;
                return;
            }

            app.modals.close(modal);
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err, false);
                app.toasts.error(err.message || "Failed to save collection group.");
            }
            data.isSubmitting = false;
        }
    }

    modal = t.div(
        {
            pbEvent: "collectionGroupUpsertModal",
            className: "modal popup sm collection-group-upsert-modal",
            inert: () => data.isSubmitting,
            onafterclose: (el) => {
                el?.remove();
            },
        },
        t.header(
            { className: "modal-header" },
            t.h5(
                { className: "m-auto txt-center" },
                settings.title || "Collection group",
            ),
        ),
        t.form(
            {
                id: uniqueId,
                className: "modal-content",
                autocomplete: "off",
                onsubmit: async (e) => {
                    e.preventDefault();
                    await submit();
                },
            },
            t.div(
                { className: "field" },
                t.label({ htmlFor: uniqueId + "_name" }, "Group name"),
                t.input({
                    id: uniqueId + "_name",
                    name: "name",
                    type: "text",
                    required: true,
                    autofocus: true,
                    value: () => data.name,
                    oninput: (e) => (data.name = e.target.value),
                }),
            ),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    disabled: () => data.isSubmitting,
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Cancel"),
            ),
            t.button(
                {
                    "html-form": uniqueId,
                    type: "submit",
                    className: () => `btn ${data.isSubmitting ? "loading" : ""}`,
                    disabled: () => !data.canSubmit,
                },
                t.span({ className: "txt" }, settings.submitLabel || "Save"),
            ),
        ),
    );

    return modal;
}
