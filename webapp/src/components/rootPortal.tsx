import {Portal} from 'solid-js/web'
import type {ParentComponent} from 'solid-js'

// Renders into #root-portal — the node index.html provides above
// the app root, which is what keeps dialogs over everything else.
const RootPortal: ParentComponent = (props) => {
    const rootPortal = document.getElementById('root-portal')

    return (
        <Portal mount={rootPortal || document.body}>
            {props.children}
        </Portal>
    )
}

export default RootPortal
