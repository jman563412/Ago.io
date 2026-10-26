// @ts-check
/** @type {HTMLCanvasElement} */
const mainCanvas = document.getElementById("maincanvas")
mainCanvas.setAttribute("width", window.innerWidth)
mainCanvas.setAttribute("height", window.innerHeight)

const canvasContext = mainCanvas.getContext("2d")



// Instantiate WS conn with the server
const socket = new WebSocket("ws://" + location.host + "/ws")
socket.onmessage = (event) => {
    // Canvas covers entire page so top left is just (0, 0), clears the defined area
    canvasContext.clearRect(0, 0, mainCanvas.width, mainCanvas.height)
    let msg = JSON.parse(event.data)

    canvasContext.beginPath()
    canvasContext.fillStyle = "red"

    // Arc only traces the path, need .fill() to actually color the path
    canvasContext.arc(msg.X, msg.Y, msg.R, 0, 2 * Math.PI)
    canvasContext.fill()

}

// Listen to client window position and send to server
window.addEventListener("mousemove", (event) => {
    if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({
            "X": event.clientX,
            "Y": event.clientY
        }))
    }
})