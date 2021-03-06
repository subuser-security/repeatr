package def

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/ugorji/go/codec"
)

//
// RunRecord
//

var _ codec.Selfer = &_assertHelper

func (rr RunRecord) CodecEncodeSelf(c *codec.Encoder) {
	// Copy pretty much the entire struct over to an anonymous one,
	//  as a way of saying "do all the normal things you would with tags";
	//  we just want to inject an Opinion on this one field.
	//
	// To be clear: this is terrible.
	//  My kingdom for a next generation of serialization/structmapping
	//  tools which actually let me pick out the *field* without this
	//  copypasta trainwreck of misplaced ambitions.
	c.MustEncode(struct {
		HID        string      `json:"HID,omitempty"`
		UID        RunID       `json:"UID"`
		Date       time.Time   `json:"when"`
		FormulaHID string      `json:"formulaHID,omitempty"`
		Results    ResultGroup `json:"results"`
		Failure    error       `json:"failure,omitempty"`
	}{
		HID:        rr.HID,
		UID:        rr.UID,
		Date:       rr.Date,
		FormulaHID: rr.FormulaHID,
		Results:    rr.Results,
		Failure: func() error {
			if rr.Failure == nil {
				return nil
			}
			return runRecordFailureEnvelope{
				failureTypeToString(rr.Failure),
				rr.Failure,
			}
		}(),
	})
}

func (rr *RunRecord) CodecDecodeSelf(c *codec.Decoder) {
	failureEnvelope := &runRecordFailureEnvelope{
		Detail: json.RawMessage{},
	}
	rusrs := struct {
		HID        string      `json:"HID,omitempty"`
		UID        RunID       `json:"UID"`
		Date       time.Time   `json:"when"`
		FormulaHID string      `json:"formulaHID,omitempty"`
		Results    ResultGroup `json:"results"`
		Failure    error       `json:"failure,omitempty"`
	}{
		Failure: failureEnvelope,
	}
	c.MustDecode(&rusrs)
	rr.HID = rusrs.HID
	rr.UID = rusrs.UID
	rr.Date = rusrs.Date
	rr.FormulaHID = rusrs.FormulaHID
	rr.Results = rusrs.Results
	if failureEnvelope.Type != "" {
		realFailure := stringToBlankFailure(failureEnvelope.Type)
		// I give up.  The following is broken:
		//  we use a json serializer here.  This will break for CBOR.
		//  I tried many other things.
		//  See the commit history immediately preceeding this area.
		//  The fix for this is *changing codec libraries entirely*.
		err := json.Unmarshal(failureEnvelope.Detail.(json.RawMessage), realFailure)
		if err != nil {
			panic(&ErrUnmarshalling{
				Msg: fmt.Sprintf("cannot unmarshal error type: %s", err),
			})
		}
		rr.Failure = realFailure
	}
}

type runRecordFailureEnvelope struct {
	Type   string      `json:"type"`
	Detail interface{} `json:"detail"`
}

func (runRecordFailureEnvelope) Error() string { return "" }

func failureTypeToString(e error) string {
	switch e.(type) {
	case *ErrConfigParsing:
		return "ErrConfigParsing"
	case *ErrConfigValidation:
		return "ErrConfigValidation"
	case *ErrWarehouseUnavailable:
		return "ErrWarehouseUnavailable"
	case *ErrWarehouseProblem:
		return "ErrWarehouseProblem"
	case *ErrWareDNE:
		return "ErrWareDNE"
	case *ErrHashMismatch:
		return "ErrHashMismatch"
	case *ErrWareCorrupt:
		return "ErrWareCorrupt"
	default:
		panic(fmt.Errorf("Internal Error type %T not suitable for API.\n\tFull error: %s", e, e))
	}
}

func stringToBlankFailure(typ string) error {
	switch typ {
	case "ErrConfigParsing":
		return &ErrConfigParsing{}
	case "ErrConfigValidation":
		return &ErrConfigValidation{}
	case "ErrWarehouseUnavailable":
		return &ErrWarehouseUnavailable{}
	case "ErrWarehouseProblem":
		return &ErrWarehouseProblem{}
	case "ErrWareDNE":
		return &ErrWareDNE{}
	case "ErrHashMismatch":
		return &ErrHashMismatch{}
	case "ErrWareCorrupt":
		return &ErrWareCorrupt{}
	default:
		panic(&ErrUnmarshalling{
			Msg: fmt.Sprintf("cannot unmarshal error type: %q is not a known type", typ),
		})
	}
}

//
// ResultGroup
//

var _ codec.Selfer = &ResultGroup{}

func (rg ResultGroup) CodecEncodeSelf(c *codec.Encoder) {
	c.Encode(rg.asMappySlice())
}

func (rg *ResultGroup) CodecDecodeSelf(c *codec.Decoder) {
	// I'd love to just punt to the defaults, but the `Selfer` interface doesn't come in half.
	// SO here's a ridiculous indirection to prance around infinite recursion.
	c.MustDecode((*map[string]*Result)(rg))
}

func (mp ResultGroup) asMappySlice() codec.MapBySlice {
	keys := make([]string, len(mp))
	var i int
	for k := range mp {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	val := make(mappySlice, len(mp)*2)
	i = 0
	for _, k := range keys {
		val[i] = k
		i++
		val[i] = mp[k]
		i++
	}
	return val
}
