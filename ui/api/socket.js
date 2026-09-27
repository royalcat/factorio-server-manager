import EventBus from "../eventBus";

const bus = new EventBus();

const ws_scheme = window.location.protocol === "https:" ? "wss" : "ws";

// The current websocket connection. Recreated on every (re)connect.
let socket = null;

// Rooms this client wants to be subscribed to. Kept across reconnects so the
// subscriptions can be re-sent once the socket (re)opens. This also makes
// subscribing before the socket is open safe.
const desiredSubscriptions = new Set();

// Send a control frame on the given socket, but only when it is actually open.
function sendControlTo(target, controls) {
    if (target && target.readyState === WebSocket.OPEN) {
        target.send(
            JSON.stringify(
                {
                    room_name: "",
                    controls: controls
                }
            )
        );
    }
}

function sendControl(controls) {
    sendControlTo(socket, controls);
}

function subscribe(room) {
    desiredSubscriptions.add(room);
    sendControl({type: "subscribe", value: room});
}

function unsubscribe(room) {
    desiredSubscriptions.delete(room);
    sendControl({type: "unsubscribe", value: room});
}

function commandSendEvent(command) {
    sendControl({type: "command", value: command});
}

// Register the event bus listeners once. They only record subscription intent
// (and send it when the socket is open), so they do not need to be removed and
// re-registered on every reconnect.
bus.on('log subscribe', () => subscribe('gamelog'));
bus.on('log unsubscribe', () => unsubscribe('gamelog'));
bus.on('server status subscribe', () => subscribe('server_status'));
bus.on('command send', commandSendEvent);

function connect() {
    const ws = new WebSocket(ws_scheme + "://" + window.location.host + "/ws");
    socket = ws;

    ws.onmessage = e => {
        const {room_name, message} = JSON.parse(e.data);
        bus.emit(room_name, message);
    }

    ws.onerror = e => {
        ws.close();
    }

    ws.onclose = e => {
        if (socket === ws) {
            socket = null;
        }
        // reconnect after 5 seconds
        setTimeout(connect, 5000);
    }

    ws.onopen = e => {
        // Re-send every subscription we want to have, so room membership
        // survives reconnects and subscribing before the socket was open.
        desiredSubscriptions.forEach(room => {
            sendControlTo(ws, {type: "subscribe", value: room});
        });
    }
}

connect();

export default bus;
