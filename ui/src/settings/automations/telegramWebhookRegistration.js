export function telegramWebhookRegistration(propsArg = {}) {
    const props = store({
        className: "",
    });

    const watchers = app.utils.extendStore(props, propsArg);
    const data = store({
        isRegistering: false,
        result: null,
        get prettyResult() {
            return JSON.stringify(data.result || {}, null, 2);
        },
    });

    async function registerWebhook() {
        if (data.isRegistering) {
            return;
        }

        data.isRegistering = true;
        data.result = null;

        try {
            data.result = await app.pb.send("/api/settings/telegram/register-webhook", {
                method: "POST",
                body: {
                    webhookURL: telegramWebhookBaseURL(),
                },
            });

            if (data.result?.ok) {
                app.toasts.success("Telegram webhook registered.");
            } else {
                app.toasts.info(data.result?.description || "Telegram rejected the webhook.");
            }
        } catch (err) {
            if (!err?.isAbort) {
                data.result = err?.response || {
                    ok: false,
                    description: err?.message || "Failed to register Telegram webhook.",
                };
                app.checkApiError(err);
            }
        }

        data.isRegistering = false;
    }

    return t.div(
        {
            className: () => `telegram-webhook-registration ${props.className || ""}`,
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "automation-workflow-webhook-copy" },
            t.code(null, telegramWebhookDisplayURL()),
            app.components.copyButton(() => telegramWebhookDisplayURL()),
        ),
        t.div(
            { className: "flex gap-5 flex-wrap m-t-sm" },
            t.button(
                {
                    type: "button",
                    className: () => `btn sm outline ${data.isRegistering ? "loading" : ""}`,
                    disabled: () => data.isRegistering,
                    onclick: registerWebhook,
                },
                t.i({ className: "ti ti-webhook", ariaHidden: true }),
                t.span({ className: "txt" }, "Register webhook"),
            ),
        ),
        () =>
            data.result
                ? t.div(
                    { className: "m-t-sm" },
                    app.components.codeBlock({
                        language: "js",
                        value: () => data.prettyResult,
                    }),
                )
                : null,
    );
}

function telegramWebhookBaseURL() {
    return `${app.utils.getApiExampleURL()}/api/automation-telegram`;
}

function telegramWebhookDisplayURL() {
    return `${telegramWebhookBaseURL()}/<access-token>`;
}
