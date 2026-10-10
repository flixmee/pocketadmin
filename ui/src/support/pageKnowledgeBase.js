import { renderKnowledgeMarkdown } from "./knowledgeAnswerMarkdown";
import { streamKnowledgeChat } from "./knowledgeChatStream";
import { openKnowledgeDocumentUploadModal } from "./knowledgeDocumentUploadModal";

const documentsRequestKey = "knowledgeBase.documents";
const uploadRequestKey = "knowledgeBase.upload";

export function pageKnowledgeBase() {
    app.store.title = "Support";

    const data = store({
        documents: [],
        isLoadingDocuments: false,
        deleting: {},
        question: "",
        isAsking: false,
        answer: "",
        sources: [],
        sessionId: "",
    });
    let pollId;
    let uploadModal;
    let chatAbortController;

    function aiSetupIssue() {
        if (app.store.isLoadingSettings) {
            return "";
        }

        const ai = app.store.settings?.ai;
        if (!ai?.enabled) {
            return "Enable an AI provider before indexing documents or asking support questions.";
        }
        if (!ai.provider) {
            return "Choose an AI provider before using the knowledge base.";
        }
        if (!ai.model?.trim()) {
            return "Choose a default AI model before asking support questions.";
        }
        if (ai.provider == "anthropic") {
            return "Anthropic does not provide embeddings. Configure a custom knowledge provider to use Support search.";
        }
        if (ai.provider == "custom" && !ai.baseURL?.trim()) {
            return "Set the custom AI provider base URL before using the knowledge base.";
        }
        if (ai.provider == "custom" && !ai.embeddingModel?.trim()) {
            return "Set the custom provider embedding model before using the knowledge base.";
        }

        return "";
    }

    async function loadDocuments(silent = false) {
        if (!silent) {
            data.isLoadingDocuments = true;
        }
        try {
            const result = await app.pb.send("/api/kb/documents", {
                method: "GET",
                requestKey: documentsRequestKey,
            });
            data.documents = result?.items || [];
        } catch (err) {
            if (!err?.isAbort && !silent) {
                app.checkApiError(err);
            }
        } finally {
            if (!silent) {
                data.isLoadingDocuments = false;
            }
        }
    }

    async function uploadDocument({ file, title }) {
        if (!file || aiSetupIssue()) {
            return false;
        }

        try {
            const body = new FormData();
            body.append("file", file);
            if (title) {
                body.append("title", title);
            }

            await app.pb.send("/api/kb/documents", {
                method: "POST",
                body,
                requestKey: uploadRequestKey,
            });
            await loadDocuments();
            app.toasts.success("Document queued for indexing.");
            return true;
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
            return false;
        }
    }

    function deleteDocument(document) {
        app.modals.confirm(
            `Delete “${document.title}” and its search index?`,
            async () => {
                data.deleting[document.id] = true;
                try {
                    await app.pb.send(`/api/kb/documents/${encodeURIComponent(document.id)}`, {
                        method: "DELETE",
                    });
                    data.documents = data.documents.filter((item) => item.id != document.id);
                    app.toasts.success("Document deleted.");
                } catch (err) {
                    app.checkApiError(err);
                    return false;
                } finally {
                    data.deleting[document.id] = false;
                }
            },
            null,
            { yesButton: "Delete", noButton: "Cancel" },
        );
    }

    async function askQuestion(event) {
        event?.preventDefault();
        const question = data.question.trim();
        if (!question || data.isAsking || aiSetupIssue()) {
            return;
        }

        data.isAsking = true;
        data.answer = "";
        data.sources = [];
        const requestController = new AbortController();
        chatAbortController?.abort();
        chatAbortController = requestController;
        try {
            const result = await streamKnowledgeChat({
                question,
                sessionId: data.sessionId,
                topK: 5,
                signal: requestController.signal,
                ontoken: (delta) => (data.answer += delta),
            });
            data.answer = result?.answer || "";
            data.sources = result?.sources || [];
            data.sessionId = result?.sessionId || data.sessionId;
            data.question = "";
        } catch (err) {
            if (err?.name != "AbortError") {
                app.checkApiError(err);
            }
        } finally {
            if (chatAbortController == requestController) {
                chatAbortController = null;
                data.isAsking = false;
            }
        }
    }

    function statusLabel(document) {
        const status = document.status || "pending";
        return t.span(
            { className: () => `label kb-status kb-status-${status}` },
            status,
        );
    }

    function documentsPanel() {
        return t.section(
            { className: "kb-panel kb-documents-panel" },
            t.div(
                { className: "kb-panel-header" },
                t.div(
                    { className: "kb-panel-heading" },
                    t.h5(null, "Knowledge documents"),
                    t.p({ className: "txt-hint" }, "Upload text, Markdown, HTML, DOCX, or text-layer PDF files."),
                ),
                t.div(
                    { className: "kb-panel-actions" },
                    t.button(
                        {
                            type: "button",
                            className: "btn outline",
                            disabled: () => !!aiSetupIssue(),
                            onclick: () => {
                                uploadModal = openKnowledgeDocumentUploadModal({
                                    onupload: uploadDocument,
                                    onclose: () => (uploadModal = null),
                                });
                            },
                        },
                        t.i({ className: "ri-add-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "Add document"),
                    ),
                    app.components.refreshButton({
                        className: "btn outline secondary circle",
                        onclick: () => loadDocuments(),
                    }),
                ),
            ),
            () => {
                if (data.isLoadingDocuments) {
                    return t.div({ className: "kb-empty" }, t.span({ className: "loader" }));
                }
                if (!data.documents.length) {
                    return t.div(
                        { className: "kb-empty" },
                        t.i({ className: "ri-file-text-line", ariaHidden: true }),
                        t.span({ className: "txt-hint" }, "No support documents yet."),
                    );
                }
                return t.div(
                    { className: "kb-document-list" },
                    ...data.documents.map((document) =>
                        t.article(
                            { className: "kb-document" },
                            t.div(
                                { className: "kb-document-icon" },
                                t.i({ className: "ri-file-text-line", ariaHidden: true }),
                            ),
                            t.div(
                                { className: "kb-document-content" },
                                t.div({ className: "txt-bold txt-ellipsis" }, document.title),
                                t.div(
                                    { className: "txt-hint txt-sm txt-ellipsis" },
                                    document.file || document.mime_type || "Document",
                                ),
                                () => {
                                    if (document.error) {
                                        return t.div({ className: "txt-danger txt-sm" }, document.error);
                                    }
                                },
                            ),
                            statusLabel(document),
                            t.button(
                                {
                                    type: "button",
                                    className: () =>
                                        `btn transparent secondary circle ${
                                            data.deleting[document.id] ? "loading" : ""
                                        }`,
                                    disabled: () => !!data.deleting[document.id],
                                    ariaLabel: app.attrs.tooltip("Delete document", "left"),
                                    onclick: () => deleteDocument(document),
                                },
                                t.i({ className: "ri-delete-bin-line", ariaHidden: true }),
                            ),
                        )
                    ),
                );
            },
        );
    }

    function chatPanel() {
        return t.section(
            { className: "kb-panel kb-chat-panel" },
            t.div(
                { className: "kb-panel-header" },
                t.div(
                    { className: "kb-panel-heading" },
                    t.h5(null, "Ask the knowledge base"),
                    t.p({ className: "txt-hint" }, "Answers include the chunks used as citations."),
                ),
                t.button(
                    {
                        type: "button",
                        className: "btn transparent secondary",
                        hidden: () => !data.sessionId,
                        disabled: () => data.isAsking,
                        onclick: () => {
                            data.sessionId = "";
                            data.answer = "";
                            data.sources = [];
                        },
                    },
                    "New chat",
                ),
            ),
            t.form(
                { className: "kb-question-form", onsubmit: askQuestion },
                t.textarea({
                    rows: 3,
                    maxlength: 8000,
                    placeholder: "Ask a support question…",
                    value: () => data.question,
                    oninput: (e) => (data.question = e.target.value),
                    onkeydown: (e) => {
                        if (e.key == "Enter" && !e.shiftKey) {
                            e.preventDefault();
                            askQuestion();
                        }
                    },
                }),
                t.button(
                    {
                        type: "submit",
                        className: () => `btn ${data.isAsking ? "loading" : ""}`,
                        disabled: () => !data.question.trim() || data.isAsking || !!aiSetupIssue(),
                    },
                    t.i({ className: "ri-send-plane-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Ask"),
                ),
            ),
            t.div(
                { className: "kb-answer", hidden: () => !data.answer && !data.isAsking },
                t.div(
                    { className: "kb-empty", hidden: () => !data.isAsking || !!data.answer },
                    t.span({ className: "loader" }),
                    "Searching and drafting…",
                ),
                t.div(
                    {
                        className: () => `kb-answer-copy ${data.isAsking ? "is-streaming" : ""}`,
                        hidden: () => !data.answer,
                    },
                    () => renderKnowledgeMarkdown(data.answer),
                ),
                t.div(
                    { className: "kb-streaming-status txt-hint txt-sm", hidden: () => !data.isAsking || !data.answer },
                    t.span({ className: "loader" }),
                    "Drafting answer…",
                ),
                () => {
                    if (data.isAsking || !data.answer) {
                        return;
                    }
                    return data.sources.length
                        ? t.div(
                            { className: "kb-sources" },
                            t.h6(null, "Sources"),
                            ...data.sources.map((source, index) =>
                                t.article(
                                    { className: "kb-source" },
                                    t.div(
                                        { className: "kb-source-title" },
                                        t.span({ className: "label" }, `[${index + 1}]`),
                                        t.strong(null, source.documentTitle),
                                        t.span({ className: "txt-hint" }, `Chunk ${source.chunkIndex + 1}`),
                                    ),
                                    t.p(null, source.content),
                                )
                            ),
                        )
                        : t.p({ className: "txt-hint" }, "No source chunks were returned.");
                },
            ),
        );
    }

    return t.div(
        {
            pbEvent: "pageKnowledgeBase",
            className: "page page-knowledge-base",
            onmount: () => {
                loadDocuments();
                pollId = setInterval(() => {
                    if (data.documents.some((item) => item.status == "pending" || item.status == "processing")) {
                        loadDocuments(true);
                    }
                }, 2000);
            },
            onunmount: () => {
                clearInterval(pollId);
                if (uploadModal) {
                    app.modals.close(uploadModal, true);
                    uploadModal = null;
                }
                app.pb.cancelRequest(documentsRequestKey);
                app.pb.cancelRequest(uploadRequestKey);
                chatAbortController?.abort();
                chatAbortController = null;
            },
        },
        t.div(
            { className: "page-content full-height" },
            t.header(
                { className: "page-header" },
                t.nav(
                    { className: "breadcrumbs" },
                    t.div({ className: "breadcrumb-item" }, "Support knowledge base"),
                ),
                t.div({ className: "flex-fill" }),
                t.a(
                    { className: "btn outline", href: "#/settings" },
                    t.i({ className: "ri-sparkling-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "AI settings"),
                ),
            ),
            t.div(
                { className: "wrapper" },
                () => {
                    const issue = aiSetupIssue();
                    if (!issue) {
                        return;
                    }

                    return t.div(
                        { className: "alert warning kb-ai-warning", role: "status" },
                        t.i({ className: "ri-alert-line", ariaHidden: true }),
                        t.div(
                            { className: "flex-fill" },
                            t.strong(null, "AI setup required"),
                            t.div({ className: "txt-sm" }, issue),
                        ),
                        t.a({ className: "btn outline warning", href: "#/settings" }, "Configure AI"),
                    );
                },
                t.div({ className: "kb-layout" }, documentsPanel(), chatPanel()),
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
