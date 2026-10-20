function rot13(str) {
  return str.replace(/[a-zA-Z]/g, (char) => {
    const base = char <= 'Z' ? 65 : 97;
    return String.fromCharCode(((char.charCodeAt(0) - base + 13) % 26) + base);
  });
}

const encoded = 'Pbatenghyngvbaf ba ohvyqvat n pbqr-rqvgvat ntrag!';
console.log(rot13(encoded));