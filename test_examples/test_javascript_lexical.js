/** Paths such as docs/* are ordinary comment contents. */
function quote(value) {
    return `"${value.replace(/(["\\])/g, "\\$1")}"`;
}
function banner() {
    const text = `art \` brace }
    still text`;
    return text;
}
function interpolation() {
    return `${ /* } ` */ 1 // } `
    }`;
}
function view() {
    return (<Text>[q]uit [/]filter</Text>);
}
function after() {}
