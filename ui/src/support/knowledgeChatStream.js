export async function streamKnowledgeChat({
    question,
    sessionId = "",
    topK = 5,
    signal,
    ontoken = null,
}) {
    const response = await fetch(app.pb.buildURL("/api/kb/chat"), {
        method: "POST",
        headers: {
            Accept: "text/event-stream",
            Authorization: app.pb.authStore.token,
            "Content-Type": "application/json",
        },
        body: JSON.stringify({
            question,
            sessionId,
            topK,
            stream: true,
        }),
        signal,
    });

    if (!response.ok) {
        throw await knowledgeChatResponseError(response);
    }
    if (!response.body) {
        throw new Error("The knowledge chat stream is not available in this browser.");
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    let result;

    while (true) {
        const chunk = await reader.read();
        buffer += decoder.decode(chunk.value || new Uint8Array(), { stream: !chunk.done });
        buffer = buffer.replaceAll("\r\n", "\n");

        let boundary;
        while ((boundary = buffer.indexOf("\n\n")) >= 0) {
            const rawEvent = buffer.slice(0, boundary);
            buffer = buffer.slice(boundary + 2);
            const event = parseKnowledgeChatEvent(rawEvent);
            if (!event) {
                continue;
            }

            if (event.name == "token") {
                const delta = event.data?.delta || "";
                if (delta) {
                    ontoken?.(delta);
                }
            } else if (event.name == "done") {
                result = event.data;
            } else if (event.name == "error") {
                throw new Error(event.data?.message || "Knowledge answer generation failed.");
            }
        }

        if (chunk.done) {
            break;
        }
    }

    if (!result) {
        throw new Error("The knowledge chat stream ended before the answer was completed.");
    }
    return result;
}

function parseKnowledgeChatEvent(rawEvent) {
    let name = "message";
    const dataLines = [];
    for (const line of rawEvent.split("\n")) {
        if (line.startsWith("event:")) {
            name = line.slice("event:".length).trim();
        } else if (line.startsWith("data:")) {
            dataLines.push(line.slice("data:".length).trim());
        }
    }
    if (!dataLines.length) {
        return null;
    }

    const rawData = dataLines.join("\n");
    try {
        return { name, data: JSON.parse(rawData) };
    } catch (err) {
        throw new Error(`Invalid knowledge chat stream event: ${err.message}`);
    }
}

async function knowledgeChatResponseError(response) {
    let data;
    try {
        data = await response.json();
    } catch {
        data = {};
    }
    const error = new Error(data?.message || `Knowledge chat request failed with status ${response.status}.`);
    error.status = response.status;
    error.response = data;
    return error;
}
