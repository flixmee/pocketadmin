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
    const uniqueId = "media_upload_" + app.utils.randomString();

    const data = store({
        activeTab: "files",
        items: [],
        urlText: "",
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
            source: "file",
            file,
            url: "",
            name: file?.name || "",
            status: "queued",
            progress: 0,
            error: "",
            xhr: null,
            fetchController: null,
        });
    }

    function createUrlQueueItem(url) {
        return store({
            id: app.utils.randomString(),
            source: "url",
            file: null,
            url,
            name: getNameFromURL(url),
            status: "queued",
            progress: 0,
            error: "",
            xhr: null,
            fetchController: null,
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

    function getImageMimeExtension(mimeType) {
        const extensions = {
            "image/avif": "avif",
            "image/gif": "gif",
            "image/jpeg": "jpg",
            "image/png": "png",
            "image/svg+xml": "svg",
            "image/webp": "webp",
        };

        return extensions[mimeType] || (mimeType || "").split("/").pop() || "jpg";
    }

    function getImageMimeFromName(name) {
        const ext = (name || "").split(".").pop()?.toLowerCase();
        const mimeTypes = {
            avif: "image/avif",
            gif: "image/gif",
            jpeg: "image/jpeg",
            jpg: "image/jpeg",
            png: "image/png",
            svg: "image/svg+xml",
            webp: "image/webp",
        };

        return mimeTypes[ext] || "";
    }

    function isImageURL(url) {
        return /\.(avif|gif|jpe?g|png|svg|webp)(?:[?#].*)?$/i.test(url);
    }

    function getNameFromURL(url, mimeType = "") {
        let name = "";

        try {
            const parsed = new URL(url);
            name = decodeURIComponent(parsed.pathname.split("/").filter(Boolean).pop() || "");
        } catch (_) {
            name = "";
        }

        name = (name || "").replace(/[\\/:*?"<>|]+/g, "_").trim();

        if (!name || !name.includes(".")) {
            name = `remote-image-${app.utils.randomString()}.${getImageMimeExtension(mimeType)}`;
        }

        return name;
    }

    function parseURLs(value) {
        const urls = [];
        const invalid = [];

        for (const rawLine of (value || "").split(/\r?\n/)) {
            const url = rawLine.trim();
            if (!url) {
                continue;
            }

            try {
                const parsed = new URL(url);
                if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
                    throw new Error("Invalid URL protocol.");
                }
                urls.push(parsed.toString());
            } catch (_) {
                invalid.push(url);
            }
        }

        return { urls, invalid };
    }

    function addURLItems() {
        const { urls, invalid } = parseURLs(data.urlText);

        if (urls.length) {
            data.items = data.items.concat(urls.map(createUrlQueueItem));
            data.urlText = "";
        }

        if (invalid.length) {
            app.toasts.error(
                `${invalid.length} URL${invalid.length === 1 ? "" : "s"} skipped because the format is invalid.`,
            );
        }

        return urls.length;
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
        item.fetchController?.abort?.();
        item.xhr?.abort?.();
        data.items = data.items.filter((entry) => entry.id !== item.id);
    }

    function abortActiveUploads() {
        for (const item of data.items) {
            item.fetchController?.abort?.();
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

    async function resolveURLFile(item) {
        if (item.file) {
            return item.file;
        }

        const controller = new AbortController();

        item.status = "fetching";
        item.progress = 0;
        item.error = "";
        item.fetchController = controller;

        try {
            const response = await fetch(item.url, {
                signal: controller.signal,
                credentials: "omit",
            });

            if (!response.ok) {
                throw new Error(`Failed to fetch image URL (${response.status}).`);
            }

            const blob = await response.blob();
            const isImage = blob.type.startsWith("image/") || isImageURL(item.url);

            if (!isImage) {
                throw new Error("URL does not point to a supported image file.");
            }

            const name = getNameFromURL(item.url, blob.type);
            const file = new File([blob], name, {
                type: blob.type || getImageMimeFromName(name) || "image/jpeg",
            });

            if (!isAllowedFile(file)) {
                throw new Error("The image type is not allowed.");
            }

            item.file = file;
            item.name = file.name;
            item.fetchController = null;

            return file;
        } catch (err) {
            item.fetchController = null;
            item.status = "error";
            if (err.name === "AbortError") {
                err.isAbort = true;
                item.error = "";
            } else {
                item.error = err instanceof TypeError
                    ? "Failed to fetch image URL. Check that the remote server allows browser access."
                    : err.message || "Failed to fetch image URL.";
                if (err instanceof TypeError) {
                    err = new Error(item.error);
                }
            }
            throw err;
        }
    }

    async function uploadItem(item) {
        const file = item.source === "url" ? await resolveURLFile(item) : item.file;

        return new Promise((resolve, reject) => {
            const formData = new FormData();
            formData.append("kind", "file");
            formData.append("parent", settings.parentId || "");
            formData.append("file", file);

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
        if (!data.isSubmitting && data.urlText.trim()) {
            addURLItems();
        }

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
        if (item.status === "fetching") {
            return "Fetching...";
        }

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

    function itemName(item) {
        return item.name || item.file?.name || item.url || "Unnamed file";
    }

    function itemDescription(item) {
        return item.error || (item.source === "url" ? item.url : app.utils.formattedFileSize(item.file?.size || 0));
    }

    function uploadButtonText() {
        if (!data.uploadableCount && data.urlText.trim()) {
            return "Upload URLs";
        }

        return `Upload ${data.uploadableCount} item${data.uploadableCount === 1 ? "" : "s"}`;
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
            { className: "tabs-header equal-width media-upload-tabs" },
            t.button(
                {
                    type: "button",
                    className: () => `tab-item ${data.activeTab === "files" ? "active" : ""}`,
                    onclick: () => (data.activeTab = "files"),
                },
                t.i({ className: "ri-folder-upload-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Files"),
            ),
            t.button(
                {
                    type: "button",
                    className: () => `tab-item ${data.activeTab === "url" ? "active" : ""}`,
                    onclick: () => (data.activeTab = "url"),
                },
                t.i({ className: "ri-links-line", ariaHidden: true }),
                t.span({ className: "txt" }, "URL"),
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
                    hidden: () => data.activeTab !== "files",
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
                    className: "media-upload-url-panel",
                    hidden: () => data.activeTab !== "url",
                },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: uniqueId + "_url_input" }, "Image URLs"),
                    t.textarea({
                        id: uniqueId + "_url_input",
                        rows: 8,
                        placeholder: "https://example.com/image.jpg",
                        spellcheck: false,
                        autocorrect: false,
                        autocomplete: "off",
                        autocapitalize: "off",
                        value: () => data.urlText,
                        oninput: (e) => (data.urlText = e.target.value),
                    }),
                    t.div(
                        { className: "field-help p-10" },
                        "Enter one image URL per line. The remote server must allow browser access.",
                    ),
                ),
                t.button(
                    {
                        type: "button",
                        className: "btn secondary",
                        disabled: () => data.isSubmitting || !data.urlText.trim(),
                        onclick: addURLItems,
                    },
                    t.i({ className: "ri-add-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Add URLs"),
                ),
            ),
            t.div(
                {
                    className: "media-upload-queue",
                    hidden: () => !data.totalCount,
                },
                t.div(
                    { className: "media-upload-queue-title" },
                    () => `${data.isSubmitting ? "Uploading" : "Items"} (${data.totalCount})`,
                ),
                t.div(
                    { className: "media-upload-queue-list" },
                    () =>
                        data.items.map((item) =>
                            t.div(
                                { className: `media-upload-item media-upload-item-${item.status}` },
                                t.div(
                                    { className: "media-upload-item-badge" },
                                    () => getFileBadge(itemName(item)),
                                ),
                                t.div(
                                    { className: "media-upload-item-main" },
                                    t.div(
                                        { className: "media-upload-item-meta" },
                                        t.div(
                                            { className: "media-upload-item-name" },
                                            () => itemName(item),
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
                                        () => itemDescription(item),
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
                    disabled: () => (!data.uploadableCount && !data.urlText.trim()) || data.isSubmitting,
                    onclick: submit,
                },
                t.span({ className: "txt" }, uploadButtonText),
            ),
        ),
    );

    return modal;
}
