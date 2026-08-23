import DefaultTheme from 'vitepress/theme-without-fonts'

// The palette is not copied here: this imports the very stylesheet the XCIII
// guide is painted with, because both guides belong to the same company and a
// second copy of the tokens would start drifting the day one of them is
// touched. The typefaces come along with it — the `url()`s inside that file
// resolve next to it, not next to us.
import '../../../guide/.vitepress/theme/custom.css'

export default DefaultTheme
