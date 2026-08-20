import type {JSX} from '@solidjs/web'

import {PropertyProps} from '../types'
import BaseTextEditor from '../baseTextEditor'

const Phone = (props: PropertyProps): JSX.Element => {
    return (
        <BaseTextEditor
            {...props}
            validator={() => true}
        />
    )
}
export default Phone
