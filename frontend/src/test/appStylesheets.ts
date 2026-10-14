// The page a component is drawn into is the app's page: the global stylesheets
// main.tsx loads, under the body class index.html gives. Without them a
// component stands in a layout the app never draws it in — a content block
// with no margin column is zero pixels wide, and nothing can be hovered.
import '../styles/variables.scss'
import '../styles/main.scss'
import '../styles/labels.scss'
import '../styles/_markdown.scss'

document.body.classList.add('app-body')
