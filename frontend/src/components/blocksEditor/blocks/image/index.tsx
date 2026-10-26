import {Show, onSettled} from 'solid-js'

import {BlockInputProps, ContentType} from '../types'
import {createFileLoader} from '../../../../hooks/fileLoader'
import {LoadError, LoadingMessage} from '../../../asyncContent'

import './image.scss'

type FileInfo = {
    file: string|File
    width?: number
    align?: 'left'|'center'|'right'
}

const Image: ContentType<FileInfo> = {
    name: 'image',
    displayName: 'Image',
    slashCommand: '/image',
    prefix: '',
    runSlashCommand: (): void => {},
    editable: false,
    Display: (props: BlockInputProps<FileInfo>) => {
        const [fileInfo, reload, loading, error] = createFileLoader(() => props.currentBoardId, () => props.value?.file)
        const imageDataUrl = () => fileInfo().url

        return (
            <>
                <Show when={loading()}><LoadingMessage/></Show>
                <Show when={error()}><LoadError onRetry={reload}/></Show>
                <Show when={imageDataUrl()}>
                    <img
                        data-testid='image'
                        class='ImageView'
                        src={imageDataUrl()!}
                    />
                </Show>
            </>
        )
    },
    Input: (props: BlockInputProps<FileInfo>) => {
        let ref: HTMLInputElement|undefined
        onSettled(() => {
            ref?.click()
        })

        return (
            <div>
                <Show when={props.value.file && (typeof props.value.file === 'string')}>
                    <img
                        class='ImageView'
                        src={props.value.file as string}
                        onClick={() => ref?.click()}
                    />
                </Show>
                <input
                    ref={ref}
                    class='Image'
                    data-testid='image-input'
                    type='file'
                    accept='image/*'
                    onChange={(e) => {
                        const file = (e.currentTarget?.files || [])[0]
                        props.onSave({file})
                    }}
                />
            </div>
        )
    },
}

Image.runSlashCommand = (changeType: (contentType: ContentType<FileInfo>) => void, changeValue: (value: FileInfo) => void): void => {
    changeType(Image)
    changeValue({file: ''})
}

export default Image
