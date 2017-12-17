rm -rf build
mkdir build
windres main.rc main.o
gcc -I ./vendor/windivert -L ./vendor/windivert/x86 -lWinDivert main.c main.o -o ./build/proxy-divert.exe 
cp ./vendor/windivert/x86/* ./build/.
rm main.o
