// FizzBuzz - A classic programming exercise
// Run with: node fizzbuzz.js

for (let i = 1; i <= 15; i++) {
  let output = "";

  if (i % 3 === 0) output += "Fizz";
  if (i % 5 === 0) output += "Buzz";

  // If neither condition matched, output the number itself
  console.log(output || i);
}