window.app = window.app || {};
window.app.modals = window.app.modals || {};

const mediasCollectionName = "_medias";

const defaultSettings = {
    parentId: "",
    title: "Upload Media",
    mimeTypes: [],
    supportText: "",
    onuploaded: function() {},
    oncomplete: function() {},
};

window.app.modals.openMediaUpload = function(settings = {}) {
    settings = Object.assign({}, defaultSettings, settings);

    const modal = mediaUploadModal(settings);
    document.body.appendChild(modal);
    app.modals.open(modal);
};

function mediaUploadModal(settings) {
    let modal;
    let dragDepth = 0;

    const data = store({
        items: [],
        isDragActive: false,
        isSubmitting: false,
        get uploadableCount() {
            return data.items.filter((item) => item.status === "queued" || item.status === "error").length;
        },
        get totalCount() {
            return data.items.length;
        },
    });

    function close() {
        app.modals.close(modal);
    }

    function buildSupportText() {
        if (settings.supportText) {
            return settings.supportText;
        }

        if (!settings.mimeTypes?.length) {
            return "Supports all allowed media file types.";
        }

        const labels = settings.mimeTypes
            .slice(0, 4)
            .map((mime) => mime.split("/").pop()?.toUpperCase() || mime.toUpperCase());

        return "Supports " + labels.join(", ");
    }

    function getFileBadge(name) {
        const ext = (name || "").split(".").pop()?.slice(0, 4) || "FILE";
        return ext.toUpperCase();
    }

    function createQueueItem(file) {
        return store({
            id: app.utils.randomString(),
            file,
            status: "queued",
            progress: 0,
            error: "",
            xhr: null,
        });
    }

    function isAllowedFile(file) {
        return !(
            settings.mimeTypes?.length
            && file?.type
            && !settings.mimeTypes.includes(file.type)
        );
    }

    function addFiles(fileList) {
        const files = Array.from(fileList || []);
        if (!files.length) {
            return;
        }

        const accepted = [];
        let skipped = 0;

        for (const file of files) {
            if (!isAllowedFile(file)) {
                skipped++;
                continue;
            }

            accepted.push(createQueueItem(file));
        }

        if (accepted.length) {
            data.items = data.items.concat(accepted);
        }

        if (skipped) {
            app.toasts.error(
                `${skipped} file${skipped === 1 ? "" : "s"} skipped because the type is not allowed.`,
            );
        }
    }

    function hasDraggedFiles(event) {
        return Array.from(event.dataTransfer?.types || []).includes("Files");
    }

    function handleDragEnter(event) {
        if (!hasDraggedFiles(event)) {
            return;
        }

        event.preventDefault();
        dragDepth += 1;
        data.isDragActive = true;
    }

    function handleDragOver(event) {
        if (!hasDraggedFiles(event)) {
            return;
        }

        event.preventDefault();
        if (event.dataTransfer) {
            event.dataTransfer.dropEffect = "copy";
        }
        data.isDragActive = true;
    }

    function handleDragLeave(event) {
        if (!hasDraggedFiles(event)) {
            return;
        }

        event.preventDefault();
        dragDepth = Math.max(0, dragDepth - 1);
        if (!dragDepth) {
            data.isDragActive = false;
        }
    }

    function handleDrop(event) {
        if (!hasDraggedFiles(event)) {
            return;
        }

        event.preventDefault();
        dragDepth = 0;
        data.isDragActive = false;
        addFiles(event.dataTransfer?.files);
    }

    function removeItem(item) {
        item.xhr?.abort?.();
        data.items = data.items.filter((entry) => entry.id !== item.id);
    }

    function abortActiveUploads() {
        for (const item of data.items) {
            item.xhr?.abort?.();
        }
    }

    function toApiError(xhr) {
        let response = {};

        try {
            response = JSON.parse(xhr.responseText || "{}");
        } catch (_) {
            response = {};
        }

        const err = new Error(response.message || xhr.statusText || "Upload failed.");
        err.status = xhr.status;
        err.response = response;
        return err;
    }

    async function uploadItem(item) {
        return new Promise((resolve, reject) => {
            const formData = new FormData();
            formData.append("kind", "file");
            formData.append("parent", settings.parentId || "");
            formData.append("file", item.file);

            const xhr = new XMLHttpRequest();

            item.status = "uploading";
            item.progress = 0;
            item.error = "";
            item.xhr = xhr;

            xhr.open(
                "POST",
                app.pb.buildURL(`/api/collections/${encodeURIComponent(mediasCollectionName)}/records`),
            );

            xhr.setRequestHeader("x-request-source", "pbui");

            if (app.pb.authStore.token) {
                xhr.setRequestHeader("Authorization", app.pb.authStore.token);
            }

            xhr.upload.addEventListener("progress", (event) => {
                if (!event.lengthComputable) {
                    return;
                }

                item.progress = Math.max(1, Math.round((event.loaded / event.total) * 100));
            });

            xhr.addEventListener("load", async () => {
                item.xhr = null;

                if (xhr.status >= 200 && xhr.status < 300) {
                    const record = JSON.parse(xhr.responseText || "{}");

                    item.progress = 100;
                    item.status = "done";

                    document.dispatchEvent(new CustomEvent("record:create", { detail: record }));
                    document.dispatchEvent(new CustomEvent("record:save", { detail: record }));

                    await Promise.resolve(settings.onuploaded?.(record));
                    resolve(record);
                    return;
                }

                item.status = "error";
                item.error = xhr.statusText || "Upload failed.";
                reject(toApiError(xhr));
            });

            xhr.addEventListener("error", () => {
                item.xhr = null;
                item.status = "error";
                item.error = "Network error.";
                reject(toApiError(xhr));
            });

            xhr.addEventListener("abort", () => {
                item.xhr = null;
                item.status = "canceled";
                item.error = "";
                resolve(null);
            });

            xhr.send(formData);
        });
    }

    async function submit() {
        if (data.isSubmitting || !data.uploadableCount) {
            return;
        }

        data.isSubmitting = true;
        app.store.errors = null;

        const pending = data.items.filter((item) => item.status === "queued" || item.status === "error");
        let uploaded = 0;
        let failed = 0;

        for (const item of pending) {
            try {
                const record = await uploadItem(item);
                if (record?.id) {
                    uploaded++;
                }
            } catch (err) {
                failed++;
                app.checkApiError(err, failed === 1);
            }
        }

        data.isSubmitting = false;

        if (uploaded && !failed) {
            app.toasts.success(`${uploaded} file${uploaded === 1 ? "" : "s"} uploaded.`);
            await Promise.resolve(settings.oncomplete?.({ uploaded, failed }));
            close();
            return;
        }

        if (uploaded || failed) {
            await Promise.resolve(settings.oncomplete?.({ uploaded, failed }));
        }
    }

    function itemStatusText(item) {
        if (item.status === "uploading") {
            return `${item.progress}%`;
        }

        if (item.status === "done") {
            return "Uploaded";
        }

        if (item.status === "error") {
            return "Failed";
        }

        return "Waiting...";
    }

    const fileInput = t.input({
        type: "file",
        hidden: true,
        multiple: true,
        accept: () => settings.mimeTypes?.join(",") || undefined,
        onchange: (e) => {
            addFiles(e.target.files);
            e.target.value = null;
        },
    });

    modal = t.div(
        {
            className: "modal popup media-upload-modal",
            onbeforeclose: () => {
                abortActiveUploads();
            },
            onafterclose: (el) => {
                el.remove();
            },
        },
        fileInput,
        t.header(
            { className: "modal-header" },
            t.h5({ className: "modal-title" }, () => settings.title || "Upload Media"),
            t.div({ className: "flex-fill" }),
            t.button(
                {
                    type: "button",
                    className: "btn sm secondary transparent circle modal-close-btn",
                    title: "Close",
                    onclick: close,
                },
                t.i({ className: "ri-close-line", ariaHidden: true }),
            ),
        ),
        t.div(
            {
                className: () =>
                    `modal-content media-upload-content ${data.isDragActive ? "media-upload-content-drag" : ""}`,
            },
            t.button(
                {
                    type: "button",
                    className: "media-upload-dropzone",
                    ondragenter: handleDragEnter,
                    ondragover: handleDragOver,
                    ondragleave: handleDragLeave,
                    ondrop: handleDrop,
                    onclick: () => fileInput.click(),
                },
                t.div(
                    { className: "media-upload-dropzone-panel" },
                    t.i({ className: "ri-upload-cloud-line", ariaHidden: true }),
                    t.div({ className: "media-upload-dropzone-title" }, "Drop files here or click to browse"),
                    t.div({ className: "media-upload-dropzone-copy" }, buildSupportText),
                ),
            ),
            t.div(
                {
                    className: "media-upload-queue",
                    hidden: () => !data.totalCount,
                },
                t.div(
                    { className: "media-upload-queue-title" },
                    () => `${data.isSubmitting ? "Uploading" : "Files"} (${data.totalCount})`,
                ),
                t.div(
                    { className: "media-upload-queue-list" },
                    () =>
                        data.items.map((item) =>
                            t.div(
                                { className: `media-upload-item media-upload-item-${item.status}` },
                                t.div(
                                    { className: "media-upload-item-badge" },
                                    () => getFileBadge(item.file?.name),
                                ),
                                t.div(
                                    { className: "media-upload-item-main" },
                                    t.div(
                                        { className: "media-upload-item-meta" },
                                        t.div(
                                            { className: "media-upload-item-name" },
                                            () => item.file?.name || "Unnamed file",
                                        ),
                                        t.div(
                                            { className: "media-upload-item-status" },
                                            () => itemStatusText(item),
                                        ),
                                    ),
                                    t.div(
                                        { className: "media-upload-item-progress" },
                                        t.div(
                                            {
                                                className: "media-upload-item-progress-bar",
                                                style: () => `width: ${item.progress || 0}%`,
                                            },
                                        ),
                                    ),
                                    t.div(
                                        { className: "media-upload-item-copy txt-hint" },
                                        () => item.error || app.utils.formattedFileSize(item.file?.size || 0),
                                    ),
                                ),
                                t.button(
                                    {
                                        type: "button",
                                        className: "btn sm secondary transparent circle",
                                        title: "Remove file",
                                        onclick: () => removeItem(item),
                                    },
                                    t.i({ className: "ri-close-circle-line", ariaHidden: true }),
                                ),
                            )
                        ),
                ),
            ),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    onclick: close,
                },
                t.span({ className: "txt" }, "Cancel"),
            ),
            t.button(
                {
                    type: "button",
                    className: () => `btn ${data.isSubmitting ? "loading" : ""}`,
                    disabled: () => !data.uploadableCount || data.isSubmitting,
                    onclick: submit,
                },
                t.span(
                    { className: "txt" },
                    () => `Upload ${data.uploadableCount} file${data.uploadableCount === 1 ? "" : "s"}`,
                ),
            ),
        ),
    );

    return modal;
}
