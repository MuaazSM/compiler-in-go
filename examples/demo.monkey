puts("--- Arithmetic ---");
puts(1 + 2 * 3);
puts((10 - 4) / 2);

puts("--- True / false and if ---");
let age = 20;
puts(age > 18);
puts(if (age > 18) { "adult" } else { "minor" });

puts("--- Text, lists and dictionaries ---");
let name = "Monkey";
puts("Hello, " + name + "!");
let numbers = [1, 2, 3, 4, 5];
puts(numbers[0]);
puts(len(numbers));
let person = {"name": "Ada", "language": "Monkey"};
puts(person["name"]);

puts("--- Functions ---");
let square = fn(x) { x * x };
puts(square(7));

puts("--- Closures: a function that remembers a value ---");
let makeAdder = fn(a) { fn(b) { a + b } };
let addTen = makeAdder(10);
puts(addTen(5));

puts("--- Recursion and built-in helpers ---");
let map = fn(arr, f) {
  let iter = fn(arr, acc) {
    if (len(arr) == 0) { acc } else { iter(rest(arr), push(acc, f(first(arr)))) }
  };
  iter(arr, []);
};
puts(map(numbers, square));

let fibonacci = fn(x) {
  if (x < 2) { x } else { fibonacci(x - 1) + fibonacci(x - 2) }
};
puts("fibonacci(20) is:");
fibonacci(20);
