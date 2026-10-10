const acceptedDocumentTypes =
    ".txt,.md,.markdown,.html,.htm,.docx,.pdf,text/plain,text/markdown,text/html,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document";

export function openKnowledgeDocumentUploadModal(settings = {}) {
    const modal = knowledgeDocumentUploadModal(settings);

    document.body.appendChild(modal);
    app.modals.open(modal);

    return modal;
}

function knowledgeDocumentUploadModal(settings) {
    const uniqueId = "knowledge_document_upload_" + app.utils.randomString();
    const data = store({
        title: "",
        selectedFileName: "",
        selectedFileSize: 0,
        isUploading: false,
    });

    let modal;
    let fileInput;
    let selectedFile;

    function selectFile() {
        if (!data.isUploading) {
            fileInput?.click();
        }
    }

    function handleFileChange(event) {
        selectedFile = event.target?.files?.[0] || null;
        data.selectedFileName = selectedFile?.name || "";
        data.selectedFileSize = selectedFile?.size || 0;

        // The File is retained above so clearing the native input allows the same
        // document to be selected again while the visible state remains reliable.
        if (event.target) {
            event.target.value = "";
        }
    }

    async function submit() {
        if (!selectedFile || data.isUploading) {
            return;
        }

        data.isUploading = true;

        try {
            const uploaded = await settings.onupload?.({
                file: selectedFile,
                title: data.title.trim(),
            });
            if (uploaded === false) {
                data.isUploading = false;
                return;
            }

            app.modals.close(modal, true);
        } catch (err) {
            data.isUploading = false;
            throw err;
        }
    }

    modal = t.div(
        {
            pbEvent: "knowledgeDocumentUploadModal",
            className: "modal popup sm kb-upload-modal",
            onbeforeclose: () => !data.isUploading,
            onafterclose: (el) => {
                settings.onclose?.();
                el?.remove();
            },
            onunmount: () => {
                selectedFile = null;
                fileInput = null;
            },
        },
        t.header(
            { className: "modal-header kb-upload-modal-header" },
            t.div(
                { className: "flex-fill" },
                t.h5({ className: "modal-title" }, "Add knowledge document"),
                t.p({ className: "txt-hint txt-sm" }, "Upload a file to make its content available to Support."),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn sm secondary transparent circle modal-close-btn",
                    ariaLabel: app.attrs.tooltip("Close"),
                    disabled: () => data.isUploading,
                    onclick: () => app.modals.close(modal),
                },
                t.i({ className: "ri-close-line", ariaHidden: true }),
            ),
        ),
        t.form(
            {
                id: uniqueId,
                className: "modal-content kb-upload-modal-content",
                onsubmit: (event) => {
                    event.preventDefault();
                    submit();
                },
            },
            t.div(
                { className: "field" },
                t.label({ htmlFor: uniqueId + "_title" }, "Title (optional)"),
                t.input({
                    id: uniqueId + "_title",
                    type: "text",
                    maxlength: 255,
                    autofocus: true,
                    placeholder: "Uses the filename when empty",
                    value: () => data.title,
                    disabled: () => data.isUploading,
                    oninput: (event) => (data.title = event.target.value),
                }),
            ),
            t.div(
                { className: "field required" },
                t.label({ htmlFor: uniqueId + "_file" }, "Document"),
                t.input({
                    id: uniqueId + "_file",
                    type: "file",
                    className: "hidden",
                    accept: acceptedDocumentTypes,
                    onmount: (el) => (fileInput = el),
                    onchange: handleFileChange,
                }),
                t.button(
                    {
                        type: "button",
                        className: () => `kb-document-picker ${data.selectedFileName ? "has-file" : ""}`,
                        ariaLabel: () =>
                            data.selectedFileName
                                ? `Change selected document, ${data.selectedFileName}`
                                : "Choose a knowledge document",
                        disabled: () => data.isUploading,
                        onclick: selectFile,
                    },
                    t.span(
                        { className: "kb-document-picker-icon" },
                        t.i({
                            className: () => data.selectedFileName ? "ri-file-check-line" : "ri-upload-cloud-2-line",
                            ariaHidden: true,
                        }),
                    ),
                    t.span(
                        { className: "kb-document-picker-copy" },
                        () =>
                            data.selectedFileName
                                ? [
                                    t.strong({ className: "txt-ellipsis" }, data.selectedFileName),
                                    t.span(
                                        { className: "txt-hint txt-sm" },
                                        app.utils.formattedFileSize(data.selectedFileSize),
                                    ),
                                ]
                                : [
                                    t.strong(null, "Choose a document"),
                                    t.span({ className: "txt-hint txt-sm" }, "Select one file from your computer"),
                                ],
                    ),
                    t.span(
                        { className: "kb-document-picker-action" },
                        () => data.selectedFileName ? "Change" : "Browse",
                    ),
                ),
                t.p(
                    { className: "txt-hint txt-sm kb-upload-support" },
                    "Accepted formats: TXT, Markdown, HTML, DOCX, and text-layer PDF.",
                ),
            ),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    disabled: () => data.isUploading,
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Cancel"),
            ),
            t.button(
                {
                    type: "submit",
                    "html-form": uniqueId,
                    className: () => `btn ${data.isUploading ? "loading" : ""}`,
                    disabled: () => !data.selectedFileName || data.isUploading,
                },
                t.i({ className: "ri-upload-cloud-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Upload document"),
            ),
        ),
    );

    return modal;
}
