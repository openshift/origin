//
// Shell for pods.
//
// Inspired by https://github.com/yudai/gotty
//
(function() {
    var initTerminal = function(ws, message) {
        hterm.defaultStorage = new lib.Storage.Local();
        hterm.defaultStorage.clear();

        term = new hterm.Terminal();

        term.getPrefs().set("send-encoding", "raw");

        term.onTerminalReady = function() {
            var io = term.io.push();

            if (!ws) {
                term.io.showOverlay(message, null);
                return;
            }

            io.onVTKeystroke = function(str) {
                ws.send("0" + str);
            };
            io.sendString = io.onVTKeystroke;

            term.installKeyboard();
        };

        term.decorate(document.getElementById("terminal"));
        return term;
    }

    var openWs = function(url, protocols, autoReconnect) {
        var ws = new WebSocket(url, protocols);

        var term, pingTimer;

        ws.onerror = function(err) {
            term = initTerminal(null, "unable to connect");
        };

        ws.onopen = function(event) {
            term = initTerminal(ws);
            pingTimer = setInterval(sendPing, 30 * 1000, ws);
        };

        ws.onmessage = function(event) {
            data = event.data.slice(1);
            switch(event.data[0]) {
            case '1':
            case '2':
            case '3':
                term.io.writeUTF8(atob(data));
                break;
            }
        };

        ws.onclose = function(event) {
            if (term) {
                term.uninstallKeyboard();
                term.io.showOverlay("Connection Closed", null);
            }
            clearInterval(pingTimer);
            if (autoReconnect > 0) {
                setTimeout(openWs, autoReconnect * 1000);
            }
        };
    }


    var sendPing = function(ws) {
        ws.send("0"); // send a zero length message to STDIN
    }

    var arguments = function() {
        var q = lib.f.parseQuery(window.location.search);
        var api = q['api']; // host + path to exec command
        if (!api || api.length === 0) {
            return {error: "no API path"};
        }
        var container = q['container']; // the container to connect to
        if (!container || container.length === 0) {
            return {error: "no container"};
        }
        var token = window.location.hash; // the api token
        if (token.length < 2) {
            return {error: "no token"};
        }
        token = token.substring(1);
        var command = q['shellcommand'];
        if (!command || command.length === 0) {
            command = "/bin/bash";
        }
        var httpsEnabled = window.location.protocol === "https:";
        var query = '?command=' + encodeURIComponent(command) + '&container='+ encodeURIComponent(container) +'&stdout=1&stdin=1&stderr=1&tty=1&access_token=' + encodeURIComponent(token);
        var url = (httpsEnabled ? 'wss://' : 'ws://') + api + query;
        var protocols = ["base64.channel.k8s.io"];
        return {
            url: url,
            protocols: protocols,
            autoReconnect: -1
        };
    }

    var args = arguments();
    if (args['error']) {
        initTerminal(false, args['error']);
        return
    }
    openWs(args.url, args.protocols, args.autoReconnect);
})()
