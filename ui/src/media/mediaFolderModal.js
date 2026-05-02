window.app = window.app || {};
window.app.modals = window.app.modals || {};

const mediasCollectionName = "_medias";

window.app.modals.openMediaFolderUpsert = function(record = null, parentId = "", modalSettings = {
    onsave: null,
}) {
    const modal = mediaFolderUpsertModal(record, parentId, modalSettings);

    document.body.appendChild(modal);

    app.modals.open(modal);
};

function mediaFolderUpsertModal(record, parentId, modalSettings) {
    let modal;

    const data = store({
        isSaving: false,
        name: record?.name || "",
        get hasChanges() {
            const initialName = record?.name || "";
            return data.name.trim() !== initialName.trim();
        },
        get title() {
            return record?.id ? "Rename folder" : "New folder";
        },
        get submitLabel() {
            return record?.id ? "Save changes" : "Create folder";
        },
    });

    async function save() {
        if (data.isSaving || !data.name.trim()) {
            return;
        }

        data.isSaving = true;
        app.store.errors = null;

        try {
            let saved;

            if (record?.id) {
                saved = await app.pb.collection(mediasCollectionName).update(record.id, {
                    name: data.name.trim(),
                });
            } else {
                saved = await app.pb.collection(mediasCollectionName).create({
                    name: data.name.trim(),
                    kind: "folder",
                    parent: parentId || "",
                });
            }

            modalSettings.onsave?.(saved, !record?.id);
            app.toasts.success(record?.id ? "Folder renamed." : "Folder created.");
            app.modals.close(modal, true);
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
                data.isSaving = false;
            }
            return;
        }

        data.isSaving = false;
    }

    modal = t.div(
        {
            pbEvent: "mediaFolderUpsertModal",
            className: "modal popup sm media-folder-modal",
            onafterclose: (el) => {
                el?.remove();
            },
        },
        t.header({ className: "modal-header" }, t.h5({ className: "m-auto" }, () => data.title)),
        t.form(
            {
                id: "mediaFolderUpsertForm",
                className: "modal-content",
                onsubmit: (e) => {
                    e.preventDefault();
                    save();
                },
            },
            t.field(
                { className: "field" },
                t.label({ htmlFor: "media-folder-name" }, "Folder name"),
                t.input({
                    id: "media-folder-name",
                    type: "text",
                    required: true,
                    maxlength: 255,
                    value: () => data.name,
                    oninput: (e) => (data.name = e.target.value),
                }),
            ),
            () => {
                const error = app.store.errors?.name?.message;
                if (error) {
                    return t.div({ className: "field-error txt-danger m-t-sm" }, error);
                }
            },
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    disabled: () => data.isSaving,
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Close"),
            ),
            t.button(
                {
                    type: "submit",
                    "html-form": "mediaFolderUpsertForm",
                    className: () => `btn ${data.isSaving ? "loading" : ""}`,
                    disabled: () => data.isSaving || !data.name.trim() || !data.hasChanges,
                },
                t.span({ className: "txt" }, () => data.submitLabel),
            ),
        ),
    );

    return modal;
}
