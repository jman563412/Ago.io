const mainCanvas = document.getElementById("maincanvas")
mainCanvas.setAttribute("width", window.innerWidth)
mainCanvas.setAttribute("height", window.innerHeight)

const canvasContext = mainCanvas.getContext("2d")

canvasContext.beginPath()
const x = window.innerWidth / 2.0
const y = window.innerHeight / 2.0
const radius = 30
// Arc only traces the path, need .fill() to actually color the path
canvasContext.arc(x, y, radius, 0, 2 * Math.PI)
canvasContext.fillStyle = "red"
canvasContext.fill()