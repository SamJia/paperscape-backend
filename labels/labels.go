package main

import (
    //"io"
    //"io/ioutil"
    "flag"
    "os"
    "bufio"
    "fmt"
    "path/filepath"
    //"net"
    //"net/http"
    //"net/http/fcgi"
    //"strconv"
    //"unicode"
    //"encoding/json"
    //"text/scanner"
    "GoMySQL"
    "runtime"
    "strings"
    //"bytes"
    //"time"
    "math"
    //"math/rand"
    //"crypto/sha1"
    //"crypto/sha256"
    //"compress/gzip"
    //"crypto/aes"
    "sort"
    //"net/smtp"
    "log"
    "xiwi"
)

var flagDB = flag.String("db", "", "MySQL database to connect to")
var flagSkipZones = flag.Bool("skip-tiles", false, "Only generate index file not tiles")
var flagMaxCores = flag.Int("cores",-1,"Max number of system cores to use, default is all of them")

func main() {
    // parse command line options
    flag.Parse()

    if flag.NArg() != 1 {
        log.Fatal("need to specify output directory prefix (with trailing /)")
    }

    // connect to the db
    db := xiwi.ConnectToDB(*flagDB)
    if db == nil {
        return
    }
    defer db.Close()

    // read in the graph
    graph := ReadGraph(db)

    // build the quad tree
    graph.BuildQuadTree()

    GenerateAllLabelZones(graph, flag.Arg(0))
}

type Paper struct {
    id      uint
    maincat string
    x       int
    y       int
    radius  int
    age     float32
    label   string
}

type PaperSortId []*Paper
func (p PaperSortId) Len() int           { return len(p) }
func (p PaperSortId) Less(i, j int) bool { return p[i].id > p[j].id }
func (p PaperSortId) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }


type Graph struct {
    papers  []*Paper
    qt      *QuadTree
    MinX, MinY, MaxX, MaxY int
    BoundsX, BoundsY int
}

func QueryCategories(db *mysql.Client, id uint) string {
    // execute the query
    query := fmt.Sprintf("SELECT maincat,allcats FROM meta_data WHERE id=%d", id)
    err := db.Query(query)
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return ""
    }

    // get result set
    result, err := db.StoreResult()
    if err != nil {
        fmt.Println("MySQL store result error;", err)
        return ""
    }

    // check if there are any results
    if result.RowCount() == 0 {
        return ""
    }

    // should be only 1 result
    if result.RowCount() != 1 {
        fmt.Println("MySQL multiple results; result count =", result.RowCount())
        return ""
    }

    // get the row
    row := result.FetchRow()
    if row == nil {
        return ""
    }

    // get the categories
    var ok bool
    var maincat string
    if row[0] != nil {
        if maincat, ok = row[0].(string); !ok { return "" }
    }
    /*
    var allcats string
    if row[1] != nil {
        if allcats, ok := row[1].(string); !ok { return "" }
    }
    */

    db.FreeResult()

    return maincat
}

func getPaperById(papers []*Paper, id uint) *Paper {
    lo := 0
    hi := len(papers) - 1
    for lo <= hi {
        mid := (lo + hi) / 2
        if id == papers[mid].id {
            return papers[mid]
        } else if id < papers[mid].id {
            hi = mid - 1
        } else {
            lo = mid + 1
        }
    }
    return nil
}

func QueryCategories2(db *mysql.Client, papers []*Paper) {
    // execute the query
    err := db.Query("SELECT id,maincat,allcats FROM meta_data")
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return
    }

    // get result set
    result, err := db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        return
    }

    // get each row from the result
    for {
        row := result.FetchRow()
        if row == nil {
            break
        }

        var ok bool
        var id uint64
        var maincat string
        //var allcats string
        if id, ok = row[0].(uint64); !ok { continue }
        if maincat, ok = row[1].(string); !ok { continue }
        //if allcats, ok = row[2].(string); !ok { continue }

        paper := getPaperById(papers, uint(id))
        if paper != nil {
            paper.maincat = maincat
        }
    }

    db.FreeResult()
}

func QueryPapers(db *mysql.Client) []*Paper {
    // count number of papers
    err := db.Query("SELECT count(id) FROM map_data")
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return nil
    }

    // get result set
    result, err := db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        return nil
    }
    row := result.FetchRow()
    if row == nil {
        fmt.Println("MySQL didn't return a row")
        return nil
    }

    // get number of papers
    var numPapers int64
    var ok bool
    if numPapers, ok = row[0].(int64); !ok {
        fmt.Println("MySQL didn't return a number")
        return nil
    }
    db.FreeResult()

    // allocate paper array
    papers := make([]*Paper, numPapers)

    // execute the query
    err = db.Query("SELECT map_data.id,map_data.x,map_data.y,map_data.r,keywords.keywords FROM map_data,keywords WHERE map_data.id = keywords.id")
    if err != nil {
        fmt.Println("MySQL query error;", err)
        return nil
    }

    // get result set
    result, err = db.UseResult()
    if err != nil {
        fmt.Println("MySQL use result error;", err)
        return nil
    }

    // get each row from the result
    index := 0
    for {
        row := result.FetchRow()
        if row == nil {
            break
        }

        var ok bool
        var id uint64
        var x, y, r int64
        var labels []byte
        if id, ok = row[0].(uint64); !ok { continue }
        if x, ok = row[1].(int64); !ok { continue }
        if y, ok = row[2].(int64); !ok { continue }
        if r, ok = row[3].(int64); !ok { continue }
        if labels, ok = row[4].([]byte); !ok { continue }
        
        var age float64 = float64(index) / float64(numPapers)
        papers[index] = MakePaper(uint(id), int(x), int(y), int(r), age, string(labels))
        index += 1
    }

    db.FreeResult()

    if int64(index) != numPapers {
        fmt.Println("could not read all papers from map_data/keywords; wanted", numPapers, "got", index)
        return nil
    }

    // Get keywords

    return papers
}

func MakePaper(id uint, x int, y int, radius int, age float64, labels string) *Paper {
    paper := new(Paper)
    paper.id = id
    paper.x = x
    paper.y = y
    paper.radius = radius
    paper.age = float32(age)
    
    // For now pick top label only
    paper.label = strings.Split(labels,",")[0]

    return paper
}

func (paper *Paper) setLabel() {


}

func ReadGraph(db *mysql.Client) *Graph {
    graph := new(Graph)

    // load positions from the data base
    graph.papers = QueryPapers(db)
    if graph.papers == nil {
        log.Fatal("could not read papers from db")
    }
    fmt.Printf("read %v papers from db\n", len(graph.papers))

    QueryCategories2(db, graph.papers)

    for _, paper := range graph.papers {
        if paper.x - paper.radius < graph.MinX { graph.MinX = paper.x - paper.radius }
        if paper.y - paper.radius < graph.MinY { graph.MinY = paper.y - paper.radius }
        if paper.x + paper.radius > graph.MaxX { graph.MaxX = paper.x + paper.radius }
        if paper.y + paper.radius > graph.MaxY { graph.MaxY = paper.y + paper.radius }
    }

    graph.BoundsX = graph.MaxX - graph.MinX
    graph.BoundsY = graph.MaxY - graph.MinY

    //for _, paper := range graph.papers {
    //    paper.setColour()
    //}

    fmt.Printf("graph has %v papers; min=(%v,%v), max=(%v,%v)\n", len(graph.papers), graph.MinX, graph.MinY, graph.MaxX, graph.MaxY)

    return graph
}

type QuadTreeNode struct {
    //Parent          *QuadTreeNode
    //SideLength      int
    Leaf            *Paper
    Q0, Q1, Q2, Q3  *QuadTreeNode
}

type QuadTree struct {
    MinX, MinY, MaxX, MaxY, MaxR  int
    Root                    *QuadTreeNode
}

func QuadTreeInsertPaper(parent *QuadTreeNode, q **QuadTreeNode, paper *Paper, MinX, MinY, MaxX, MaxY int) {
    if *q == nil {
        // hit an empty node; create a new leaf cell and put this paper in it
        *q = new(QuadTreeNode)
        //(*q).Parent = parent
        //(*q).SideLength = MaxX - MinX
        (*q).Leaf = paper

    } else if (*q).Leaf != nil {
        // hit a leaf; turn it into an internal node and re-insert the papers
        oldPaper := (*q).Leaf
        (*q).Leaf = nil
        (*q).Q0 = nil
        (*q).Q1 = nil
        (*q).Q2 = nil
        (*q).Q3 = nil
        QuadTreeInsertPaper(parent, q, oldPaper, MinX, MinY, MaxX, MaxY)
        QuadTreeInsertPaper(parent, q, paper, MinX, MinY, MaxX, MaxY)

    } else {
        // hit an internal node

        // check cell size didn't get too small
        if (MaxX <= MinX + 1 || MaxY <= MinY + 1) {
            log.Println("ERROR: QuadTreeInsertPaper hit minimum cell size")
            return
        }

        // compute the dividing x and y positions
        MidX := (MinX + MaxX) / 2
        MidY := (MinY + MaxY) / 2

        // insert the new paper in the correct cell
        if ((paper.y) < MidY) {
            if ((paper.x) < MidX) {
                QuadTreeInsertPaper(*q, &(*q).Q0, paper, MinX, MinY, MidX, MidY)
            } else {
                QuadTreeInsertPaper(*q, &(*q).Q1, paper, MidX, MinY, MaxX, MidY)
            }
        } else {
            if ((paper.x) < MidX) {
                QuadTreeInsertPaper(*q, &(*q).Q2, paper, MinX, MidY, MidX, MaxY)
            } else {
                QuadTreeInsertPaper(*q, &(*q).Q3, paper, MidX, MidY, MaxX, MaxY)
            }
        }
    }
}

func (graph *Graph) BuildQuadTree() {
    qt := new(QuadTree)

    // if no papers, return
    if len(graph.papers) == 0 {
        return
    }

    // first work out the bounding box of all papers
    qt.MinX = graph.papers[0].x
    qt.MinY = graph.papers[0].y
    qt.MaxX = graph.papers[0].x
    qt.MaxY = graph.papers[0].y
    qt.MaxR = graph.papers[0].radius
    for _, paper := range graph.papers {
        if (paper.x < qt.MinX) { qt.MinX = paper.x; }
        if (paper.y < qt.MinY) { qt.MinY = paper.y; }
        if (paper.x > qt.MaxX) { qt.MaxX = paper.x; }
        if (paper.y > qt.MaxY) { qt.MaxY = paper.y; }
        if (paper.radius > qt.MaxR) { qt.MaxR = paper.radius; }
    }

    // increase the bounding box so it's square
    {
        dx := qt.MaxX - qt.MinX
        dy := qt.MaxY - qt.MinY
        if dx > dy {
            cen_y := (qt.MinY + qt.MaxY) / 2
            qt.MinY = cen_y - dx / 2
            qt.MaxY = cen_y + dx / 2
        } else {
            cen_x := (qt.MinX + qt.MaxX) / 2
            qt.MinX = cen_x - dy / 2
            qt.MaxX = cen_x + dy / 2
        }
    }

    // build the quad tree
    for _, paper := range graph.papers {
        QuadTreeInsertPaper(nil, &qt.Root, paper, qt.MinX, qt.MinY, qt.MaxX, qt.MaxY)
    }

    fmt.Printf("quad tree bounding box: (%v,%v) -- (%v,%v)\n", qt.MinX, qt.MinY, qt.MaxX, qt.MaxY)

    // store the quad tree in the graph object
    graph.qt = qt
}

func (q *QuadTreeNode) ApplyIfWithin(MinX, MinY, MaxX, MaxY int, x, y, rx, ry int, f func(paper *Paper)) {
    if q == nil {
    } else if q.Leaf != nil {
        rx += q.Leaf.radius
        ry += q.Leaf.radius
        if x - rx <= q.Leaf.x && q.Leaf.x <= x + rx && y - ry <= q.Leaf.y && q.Leaf.y <= y + ry {
            f(q.Leaf)
        }
    } else if ((MinX <= x - rx && x - rx <= MaxX) || (MinX <= x + rx && x + rx <= MaxX) || (x - rx <= MinX && x + rx >= MaxX)) &&
              ((MinY <= y - ry && y - ry <= MaxY) || (MinY <= y + ry && y + ry <= MaxY) || (y - ry <= MinY && y + ry >= MaxY)) {
        MidX := (MinX + MaxX) / 2
        MidY := (MinY + MaxY) / 2
        q.Q0.ApplyIfWithin(MinX, MinY, MidX, MidY, x, y, rx, ry, f)
        q.Q1.ApplyIfWithin(MidX, MinY, MaxX, MidY, x, y, rx, ry, f)
        q.Q2.ApplyIfWithin(MinX, MidY, MidX, MaxY, x, y, rx, ry, f)
        q.Q3.ApplyIfWithin(MidX, MidY, MaxX, MaxY, x, y, rx, ry, f)
    }
}

func (qt *QuadTree) ApplyIfWithin(x, y, rx, ry int, f func(paper *Paper)) {
    qt.Root.ApplyIfWithin(qt.MinX, qt.MinY, qt.MaxX, qt.MaxY, x, y, rx, ry, f)
}

func GenerateLabelZone(graph *Graph, width, height, xi, yi int, filename string) {

    if err := os.MkdirAll(filepath.Dir(filename),0755); err != nil {
        log.Fatal(err)
    }

    fo, _ := os.Create(filename+".json")
    defer fo.Close()
    w := bufio.NewWriter(fo)

    // Get midpoint of zone
    rx := float64(width/2)
    ry := float64(height/2)
    x  := float64(1-xi)*float64(width) + rx
    y  := float64(1-yi)*float64(height) + ry

    // need to add largest radius to dimensions to ensure we don't miss any papers

    // typical scale
    scale := int(math.Sqrt(float64(width)*float64(width) + float64(height)*float64(height))*0.01)
    
    // TODO consider adding depth, x, y, width, height etc.
    // Tho in practice should already have this info before d/l label zone
    fmt.Fprintf(w,"{\"scale\":%d,\"lbls\":[",scale)
    
    first := true
    graph.qt.ApplyIfWithin(int(x), int(y), int(rx), int(ry), func(paper *Paper) {
        if paper.label != "" && paper.radius > scale {
            if first {
                first = false
            } else {
                fmt.Fprintf(w,",")
            }
            fmt.Fprintf(w,"{\"x\":%d,\"y\":%d,\"lbl\":\"%s\"}",paper.x,paper.y,paper.label)
        }
    })

    fmt.Fprintf(w,"]}")
    
    //err:= png.Encode(w, surf.GetImage())
    //if err != nil {
    //    fmt.Println(err)
    //}
    w.Flush()
}

func ParallelGenerateLabelZone(graph *Graph, outPrefix string, depth, worldDim, xiFirst, xiLast, yiFirst, yiLast int, channel chan int) {
    for xi := xiFirst; xi <= xiLast; xi++ {
        for yi := yiFirst; yi <= yiLast; yi++ {
            //filename := fmt.Sprintf("%stiles/%d-%d/tile_%d-%d_%d-%d.png",outPrefix,divs,divs,divs,divs,xi,yi)
            filename := fmt.Sprintf("%s/zones/%d/%d/%d", outPrefix, depth, xi, yi)
            GenerateLabelZone(graph, worldDim, worldDim, xi, yi,filename)
        }
    }
    channel <- 1 // signal that this set of tiles is done
}

func GenerateAllLabelZones(graph *Graph, outPrefix string) {
    indexFile := outPrefix + "/zones/label_index.json"
    if err := os.MkdirAll(filepath.Dir(indexFile),0755); err != nil {
        fmt.Println(err)
        return
    }
    fo, _ := os.Create(indexFile)
    defer fo.Close()
    w := bufio.NewWriter(fo)

    sort.Sort(PaperSortId(graph.papers))
    latestId := graph.papers[0].id

    fmt.Fprintf(w,"{\"latestid\":%d,\"xmin\":%d,\"ymin\":%d,\"xmax\":%d,\"ymax\":%d,\"zones\":[",latestId,graph.MinX,graph.MinY,graph.MaxX,graph.MaxY,)

    divisionSet := [...]int{4,8,24,72,216}
    //divisionSet := [...]int{4,8,24}

    first := true
    
    for depth, divs := range divisionSet {
        //divs := int(math.Pow(2.,float64(depth)))
        worldDim := int(math.Max(float64(graph.BoundsX)/float64(divs), float64(graph.BoundsY)/float64(divs)))

        if !first {
             fmt.Fprintf(w,",")
        }
        first = false
        fmt.Fprintf(w,"{\"z\":%d,\"w\":%d,\"h\":%d,\"nx\":%d,\"ny\":%d}",depth, worldDim, worldDim, divs,divs)

        if !*flagSkipZones {
            fmt.Printf("Generating label zones at depth %d\n",divs)
            // TODO if graph far from from square, shorten tile directions accordingly

            // parallelise the drawing of zones, using as many cpus as we have available to us
            maxCpu := runtime.NumCPU()
            if *flagMaxCores > 0 && *flagMaxCores < maxCpu {
                maxCpu = *flagMaxCores
            }
            runtime.GOMAXPROCS(maxCpu)
            channel := make(chan int, maxCpu)
            numRoutines := 0
            xiPerCpu := (divs + maxCpu - 1) / maxCpu
            for xi := 1; xi <= divs; {
                xiLast := xi + xiPerCpu - 1
                if xiLast > divs {
                    xiLast = divs
                }
                go ParallelGenerateLabelZone(graph, outPrefix, depth, worldDim, xi, xiLast, 1, divs, channel)
                numRoutines += 1
                xi = xiLast + 1
            }
            // drain the channel
            for i := 0; i < numRoutines; i++ {
                <-channel // wait for one task to complete
            }
            // all tasks are finished
        }
    }
    fmt.Fprintf(w,"]}")
    w.Flush()
}