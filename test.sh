if [ $# != 1 ] ; then
echo "USAGE: $0 COUNT"
echo " e.g.: $0 1000"
exit 1;
fi

processors="$(cat /proc/cpuinfo | grep 'physical id' | wc -l)"
workers="$(($processors*3))"

echo "Start Large Scale ($1 cases, $workers workers) (Parallel (Disabled Logging)) > $file-ls.log"
LANG=en-us date
file="$(date "+%Y-%m-%d-%H-%M-%S")"

LANG=en-us date
python -u ./batch_test.py all -c $1 -w $workers -f "" > ./logs/$file-ls.log 2>&1
cat ./logs/$file-ls.log | tail -n 17

echo ""
echo "Finish"
LANG=en-us date