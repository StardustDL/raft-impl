echo "Start"
LANG=en-us date
file="$(date "+%Y-%m-%d-%H-%M-%S")"

echo ""
echo "Serial (Enabled Logging)"
LANG=en-us date
python -u ./batch_test.py all -c 100 -w 1 > ./logs/$file-sl.log 2>&1
cat ./logs/$file-sl.log | tail -n 17

echo ""
echo "Serial (Disabled Logging)"
LANG=en-us date
python -u ./batch_test.py all -c 100 -f "" -w 1 > ./logs/$file-sn.log 2>&1
cat ./logs/$file-sn.log | tail -n 17

echo ""
echo "Parallel (Enabled Logging)"
LANG=en-us date
python -u ./batch_test.py all -c 1000 > ./logs/$file-pl.log 2>&1
cat ./logs/$file-pl.log | tail -n 17

echo ""
echo "Parallel (Disabled Logging)"
LANG=en-us date
python -u ./batch_test.py all -c 1000 -f "" > ./logs/$file-pn.log 2>&1
cat ./logs/$file-pn.log | tail -n 17

echo ""
echo "Finish"
LANG=en-us date