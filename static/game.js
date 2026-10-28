// @ts-check
const mainCanvas = /** @type {HTMLCanvasElement} */ (document.getElementById("maincanvas"))

mainCanvas.width = window.innerWidth
mainCanvas.height = window.innerHeight

const canvasContext = mainCanvas.getContext("2d")
if (!canvasContext) throw new Error("Unable to get 2d canvas context")

// Instantiate WS conn with the server
const socket = new WebSocket("ws://" + location.host + "/ws")
socket.addEventListener("message", (event) => {
    // Reset offest from prev setTransform
    canvasContext.setTransform(1, 0, 0, 1, 0, 0)
    // Canvas covers entire page so top left is just (0, 0), clears the defined area
    // Clear is the one thing that shouldn't be translated, needs to cover the real screen
    canvasContext.clearRect(0, 0, mainCanvas.width, mainCanvas.height)
    
    const msg = JSON.parse(event.data)

    // For inner world, border and circle, translate canvas such that circle remains centered
    const midpointX = window.innerWidth / 2
    const midpointY = window.innerHeight / 2
    // Circle position now starts from msg.X + (center - msg.X) = center, same for Y, always centered
    // Border position now starts from center - msg.X, same for Y, same idea for outer map
    canvasContext.setTransform(1, 0, 0, 1, midpointX - msg.X, midpointY - msg.Y)


    // Use different color for outside of map
    canvasContext.fillStyle = "#1e1e1e"
    canvasContext.fillRect(0, 0, 2000, 2000)

    // World border
    canvasContext.strokeStyle = "white"
    canvasContext.strokeRect(0, 0, 2000, 2000)

    // Drawing/filling circle
    canvasContext.beginPath()
    canvasContext.fillStyle = "red"

    // Arc only traces the path, need .fill() to actually color the path
    canvasContext.arc(msg.X, msg.Y, msg.R, 0, 2 * Math.PI)
    canvasContext.fill()

})

// Listen to client window position and send to server
window.addEventListener("mousemove", (event) => {
    if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({
            "X": event.clientX,
            "Y": event.clientY
        }))
    }
})