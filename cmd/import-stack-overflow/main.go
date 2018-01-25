package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kjk/programming-books/pkg/mdutil"
	"github.com/kjk/u"
	blackfriday "gopkg.in/russross/blackfriday.v2"
)

var (
	gDocTags        []DocTag
	gTopics         []Topic
	gExamples       []*Example
	gTopicHistories []TopicHistory
	currDefaultLang string

	// if true, we cleanup markdown => markdown
	// unfortunately it seems to introduce glitches (e.g. in jQuery book)
	reformatMarkdown = false

	emptyExamplexs []*Example
	// if true, prints more information
	verbose = false

	booksToImport = mdutil.BooksToProcess
)

func mdToHTML(d []byte) []byte {
	//r := blackfriday.NewHTMLRenderer()
	return blackfriday.Run(d)
}

func mdFmt(src []byte, defaultLang string) ([]byte, error) {
	opts := &mdutil.Options{DefaultLang: defaultLang}
	return mdutil.Process(src, opts)
}

func calcExampleCount(docTag *DocTag) {
	docID := docTag.Id
	topics := make(map[int]bool)
	for _, topic := range gTopics {
		if topic.DocTagId == docID {
			topics[topic.Id] = true
		}
	}
	n := 0
	for _, ex := range gExamples {
		if topics[ex.DocTopicId] {
			n++
		}
	}
	docTag.ExampleCount = n
}

func printDocTagsMust() {
	loadAll()
	for i := range gDocTags {
		docTag := &gDocTags[i]
		calcExampleCount(docTag)
	}
	sort.Slice(gDocTags, func(i, j int) bool {
		return gDocTags[i].ExampleCount < gDocTags[j].ExampleCount
	})
	for _, dc := range gDocTags {
		fmt.Printf(`{ "%s", "", false, %d, %d },%s`, dc.Title, dc.ExampleCount, dc.TopicCount, "\n")
	}
}

func loadDocTagsMust() []DocTag {
	path := path.Join("stack-overflow-docs-dump", "doctags.json.gz")
	docTags, err := loadDocTags(path)
	u.PanicIfErr(err)
	return docTags
}

func loadTopicsMust() []Topic {
	path := path.Join("stack-overflow-docs-dump", "topics.json.gz")
	topics, err := loadTopics(path)
	u.PanicIfErr(err)
	return topics
}

func loadTopicHistoriesMust() []TopicHistory {
	path := path.Join("stack-overflow-docs-dump", "topichistories.json.gz")
	topicHistories, err := loadTopicHistories(path)
	u.PanicIfErr(err)
	return topicHistories
}

func loadExamplesMust() []*Example {
	path := path.Join("stack-overflow-docs-dump", "examples.json.gz")
	examples, err := loadExamples(path)
	u.PanicIfErr(err)
	return examples
}

func findDocTagByTitleMust(docTags []DocTag, title string) DocTag {
	for _, dc := range docTags {
		if dc.Title == title {
			return dc
		}
	}
	log.Fatalf("Didn't find DocTag with title '%s'\n", title)
	return DocTag{}
}

func loadAll() {
	timeStart := time.Now()
	gDocTags = loadDocTagsMust()
	gTopics = loadTopicsMust()
	gExamples = loadExamplesMust()
	gTopicHistories = loadTopicHistoriesMust()
	fmt.Printf("loadAll took %s\n", time.Since(timeStart))
}

func getTopicsByDocTagID(docTagID int) []*Topic {
	var res []*Topic
	for i, topic := range gTopics {
		if topic.DocTagId == docTagID {
			res = append(res, &gTopics[i])
		}
	}
	return res
}

func getExampleByID(id int) *Example {
	for i, e := range gExamples {
		if e.Id == id {
			return gExamples[i]
		}
	}
	return nil
}

func getExamplesForTopic(docTagID int, docTopicID int) []*Example {
	var res []*Example
	seenIds := make(map[int]bool)
	for _, th := range gTopicHistories {
		if th.DocTagId == docTagID && th.DocTopicId == docTopicID {
			id := th.DocExampleId
			if seenIds[id] {
				continue
			}
			seenIds[id] = true
			ex := getExampleByID(id)
			if ex == nil {
				//fmt.Printf("Didn't find example, docTagID: %d, docTopicID: %d\n", docTagID, docTopicID)
			} else {
				res = append(res, ex)
			}
		}
	}
	return res
}

func sortExamples(a []*Example) {
	sort.Slice(a, func(i, j int) bool {
		if a[i].IsPinned {
			return true
		}
		if a[j].IsPinned {
			return false
		}
		return a[i].Score > a[j].Score
	})
}

func serFitsOneLine(s string) bool {
	if len(s) > 80 {
		return false
	}
	if strings.Contains(s, "\n") {
		return false
	}
	// to avoid ambiguity when parsing serialize values with ':" on separate lines
	if strings.Contains(s, ":") {
		return false
	}
	return true
}

func isEmptyString(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) == 0
}

func serField(k, v string) string {
	if isEmptyString(v) {
		return ""
	}
	if serFitsOneLine(v) {
		return fmt.Sprintf("%s: %s\n", k, v)
	}
	u.PanicIf(strings.Contains(v, mdutil.KVRecordSeparator), "v contains KVRecordSeparator")

	return fmt.Sprintf("%s:\n%s\n%s\n", k, v, mdutil.KVRecordSeparator)
}

func serFieldMarkdown(k, v string) string {
	if isEmptyString(v) {
		return ""
	}
	if reformatMarkdown {
		d, err := mdFmt([]byte(v), currDefaultLang)
		u.PanicIfErr(err)
		return serField(k, string(d))
	}
	return serField(k, v)
}

func shortenVersion(s string) string {
	if s == "[]" {
		return ""
	}
	return s
}

func writeIndexTxtMust(path string, topic *Topic) {
	s := serField("Title", topic.Title)
	versions := shortenVersion(topic.VersionsJson)
	s += serField("Versions", versions)
	if isEmptyString(versions) {
		s += serField("VersionsHtml", topic.HelloWorldVersionsHtml)
	}

	s += serFieldMarkdown("Introduction", topic.IntroductionMarkdown)
	if isEmptyString(topic.IntroductionMarkdown) {
		s += serField("IntroductionHtml", topic.IntroductionHtml)
	}

	s += serFieldMarkdown("Syntax", topic.SyntaxMarkdown)
	if isEmptyString(topic.SyntaxMarkdown) {
		s += serField("SyntaxHtml", topic.SyntaxHtml)
	}

	s += serFieldMarkdown("Parameters", topic.ParametersMarkdown)
	if isEmptyString(topic.ParametersMarkdown) {
		s += serField("ParametersHtml", topic.ParametersHtml)
	}

	s += serFieldMarkdown("Remarks", topic.RemarksMarkdown)
	if isEmptyString(topic.RemarksMarkdown) {
		s += serField("RemarksHtml", topic.RemarksHtml)
	}

	createDirForFileMust(path)
	err := ioutil.WriteFile(path, []byte(s), 0644)
	u.PanicIfErr(err)
	if verbose {
		fmt.Printf("Wrote %s, %d bytes\n", path, len(s))
	}
}

func writeSectionMust(path string, example *Example) {
	s := serField("Title", example.Title)
	s += serField("Score", strconv.Itoa(example.Score))
	s += serFieldMarkdown("Body", example.BodyMarkdown)
	if isEmptyString(example.BodyMarkdown) {
		s += serField("BodyHtml", example.BodyHtml)
	}

	createDirForFileMust(path)
	err := ioutil.WriteFile(path, []byte(s), 0644)
	u.PanicIfErr(err)
	if verbose {
		fmt.Printf("Wrote %s, %d bytes\n", path, len(s))
	}
}

func printEmptyExamples() {
	for _, ex := range emptyExamplexs {
		fmt.Printf("empty example: %s, len(BodyHtml): %d\n", ex.Title, len(ex.BodyHtml))
	}
}

func genBook(book *mdutil.Book, defaultLang string) {
	timeStart := time.Now()
	name := book.Name
	newName := book.NewName()
	currDefaultLang = defaultLang
	bookDstDir := mdutil.MakeURLSafe(newName)
	docTag := findDocTagByTitleMust(gDocTags, name)
	//fmt.Printf("%s: docID: %d\n", title, docTag.Id)
	topics := getTopicsByDocTagID(docTag.Id)
	nChapters := len(topics)
	nSections := 0
	chapter := 10
	for _, t := range topics {
		examples := getExamplesForTopic(docTag.Id, t.Id)
		sortExamples(examples)

		dirChapter := fmt.Sprintf("%04d-%s", chapter, mdutil.MakeURLSafe(t.Title))
		dirPath := filepath.Join("books", bookDstDir, dirChapter)
		chapterIndexPath := filepath.Join(dirPath, "index.txt")
		writeIndexTxtMust(chapterIndexPath, t)
		//fmt.Printf("%s\n", dirChapter)
		chapter += 10
		//fmt.Printf("%s, %d examples (%d), %s\n", t.Title, t.ExampleCount, len(examples), fileName)

		section := 10
		for _, ex := range examples {
			if isEmptyString(ex.BodyMarkdown) && isEmptyString(ex.BodyHtml) {
				emptyExamplexs = append(emptyExamplexs, ex)
				continue
			}
			fileName := fmt.Sprintf("%03d-%s.md", section, mdutil.MakeURLSafe(ex.Title))
			path := filepath.Join(dirPath, fileName)
			writeSectionMust(path, ex)
			//fmt.Printf("  %s %s '%s'\n", ex.Title, pinnedStr, fileName)
			//fmt.Printf("  %03d-%s\n", section, fileName)
			//fmt.Printf("  %s\n", fileName)
			section += 10
		}
		nSections += len(examples)
	}
	fmt.Printf("Imported %s (%d chapters, %d sections) in %s\n", name, nChapters, nSections, time.Since(timeStart))
}

func main() {
	if false {
		printDocTagsMust()
		return
	}
	timeStart := time.Now()
	loadAll()
	for _, bookInfo := range booksToImport {
		if !bookInfo.Import {
			continue
		}
		genBook(bookInfo, "")
	}
	fmt.Printf("Took %s\n", time.Since(timeStart))
	printEmptyExamples()
}