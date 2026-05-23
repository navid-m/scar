cargo build > /dev/null 2>&1
./target/debug/scar ./self/main.scar -o scar-dev

echo "Build done."
echo
echo "Reference compiler   -> ./target/debug/scar"
echo "Self hosted compiler -> ./scar-dev"